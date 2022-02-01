package sqlfsync

import (
	"errors"
	"log"
	"reflect"

	"github.com/fsnotify/fsnotify"
	"gorm.io/gorm"
)

type SqlFSync struct {
	// pointer to database connection
	db *gorm.DB

	// list of directories to watch
	watches []WatchEntry
}

type WatchEntry struct {
	// full path to directory
	path string

	// Pointer to struct that represents
	// the entity in the database
	model interface{}

	fswatcher *fsnotify.Watcher
}

func New(db *gorm.DB) *SqlFSync {
	return &SqlFSync{db: db}
}

func (sfs *SqlFSync) Close() {
	for _, we := range sfs.watches {
		we.fswatcher.Close()
	}
}

func (sfs *SqlFSync) AddWatch(path string, model interface{}) error {

	// model argument must be a pointer to a struct
	v := reflect.ValueOf(model)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return errors.New("model argument must be a pointer to a struct")
	}

	// Check the model has a "Path" field,
	// or a struct field tag indicating
	// which field holds the file path
	e := v.Elem()
	t := e.Type()
	var structFieldName string
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag
		if tv, ok := tag.Lookup("sqlfsync"); ok && tv == "path" {
			structFieldName = t.Field(i).Name
		}
	}

	fieldType, _ := t.FieldByName(structFieldName)
	if structFieldName == "" || fieldType.Type.Kind() != reflect.String {
		return errors.New("model must have struct tag sqlfsync:\"path\" with type string")
	}

	// Create fsnotify.Watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	watcher.Add(path)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					continue
				}
				log.Println("event:", event)
				if event.Op == fsnotify.Create {
					log.Println("created file:", event.Name)

					// Create copy of model and set "path" field to event.Name
					new := reflect.New(t)
					new.Elem().FieldByName(structFieldName).SetString(event.Name)

					// Insert into database
					tx := sfs.db.Create(new.Interface())

					// What if there is an error?
					if tx.Error != nil {
						log.Println(tx.Error)
					}

				} else if event.Op == fsnotify.Remove {
					log.Println("removed file: ", event.Name)
					// Create filt copy of model for filtering purposes
					filt := reflect.New(t)
					filt.Elem().FieldByName(structFieldName).SetString(event.Name)

					// Create copy of model to put to-be-deleted entry into
					toDelete := reflect.New(t).Interface()
					
					sfs.db.Where(filt).Find(toDelete)

					sfs.db.Delete(toDelete)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					continue
				}
				log.Println("error:", err)
			}
		}
	}()

	watch := WatchEntry{path: path, model: model, fswatcher: watcher}
	sfs.watches = append(sfs.watches, watch)

	return nil
}
