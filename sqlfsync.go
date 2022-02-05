package sqlfsync

import (
	"errors"
	"log"
	"reflect"

	"github.com/fsnotify/fsnotify"
	"gorm.io/gorm"
)

// SqlFSync watches a set of directories for created or deleted files,
// and inserts or deletes from the database, respectively.
type SqlFSync struct {
	// pointer to database connection
	DB *gorm.DB

	// list of directories to watch
	Watches []WatchEntry
}

// WatchEntry represents a single directory 
type WatchEntry struct {
	// full path to directory
	Path string

	// Pointer to struct that represents
	// the entity in the database
	Model interface{}

	// Watches Path for create or delete events
	FSWatcher *fsnotify.Watcher
}

// Create a new instance of SqlFSync
func New(db *gorm.DB) *SqlFSync {
	return &SqlFSync{DB: db}
}

// Stop watching all directories 
func (sfs *SqlFSync) Close() {
	for _, we := range sfs.Watches {
		we.FSWatcher.Close()
	}
}

// Start watching the specified path for create or delete events.
// The model argument must be a pointer to a struct.
// It must have a field tagged with sqlfsync:"path".
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
					tx := sfs.DB.Create(new.Interface())

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

					sfs.DB.Where(filt).Find(toDelete)

					sfs.DB.Delete(toDelete)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					continue
				}
				log.Println("error:", err)
			}
		}
	}()

	watch := WatchEntry{Path: path, Model: model, FSWatcher: watcher}
	sfs.Watches = append(sfs.Watches, watch)

	return nil
}
