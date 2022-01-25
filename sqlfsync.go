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
	db      *gorm.DB

	// list of directories to watch
	watches []WatchEntry
}

type WatchEntry struct {
	// full path to directory
	path      string

	// Pointer to struct that represents
	// the entity in the database
	model     interface{}

	fswatcher *fsnotify.Watcher
}

func New(db *gorm.DB) *SqlFSync {
	return &SqlFSync{db: db}
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
	t := v.Elem().Type()
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
				if event.Op&fsnotify.Create == fsnotify.Create {
					log.Println("created file:", event.Name)

					// Create copy of model
					model := model

					// Set the "Path" field to the created file.
					// If "Path" is not a field, check for a field tag
					reflect.ValueOf(model).Elem().FieldByName(structFieldName).SetString(event.Name)

					// Insert into database
					tx := sfs.db.Create(model)

					// What if there is an error?
					if tx.Error != nil {
						log.Println(tx.Error)
					}

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
