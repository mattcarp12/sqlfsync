package sqlfsync_test

import (
	"os"
	"testing"
	"time"

	"github.com/mattcarp12/sqlfsync"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func NewSqliteInMemoryDB() *gorm.DB {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	return db
}

func TestValidModelStruct(t *testing.T) {
	sfs := sqlfsync.New(NewSqliteInMemoryDB())

	tempDir := t.TempDir()

	// Attemp to pass an int
	err := sfs.AddWatch(tempDir, 10)
	t.Log(err)
	if err == nil {
		t.Error("This should be an error")
	}

	type TestStruct1 struct {
		foo int
		bar string
	}

	// Attempt to pass a struct (not pointer)
	err = sfs.AddWatch(tempDir, TestStruct1{})
	t.Log(err)
	if err == nil {
		t.Error("This should be an error")
	}

	// Attemp to pass struct without proper tag
	err = sfs.AddWatch(tempDir, &TestStruct1{})
	t.Log(err)
	if err == nil {
		t.Error("This should be an error")
	}

	type TestStruct2 struct {
		foo int `sqlfsync:"path"`
		bar string
	}

	// Tagged field has wrong type
	err = sfs.AddWatch(tempDir, &TestStruct2{})
	t.Log(err)
	if err == nil {
		t.Error("This should be an error")
	}

	type TestStruct3 struct {
		foo int
		bar string `sqlfsync:"path"`
	}

	// Finally, test correct struct
	err = sfs.AddWatch(tempDir, &TestStruct3{})
	t.Log(err)
	if err != nil {
		t.Error("This should be not an error")
	}
}

type File struct {
	Path      string `sqlfsync:"path"`
	CreatedAt time.Time
}

func TestAddWatch(t *testing.T) {
	db := NewSqliteInMemoryDB()
	db.AutoMigrate(&File{})
	sfs := sqlfsync.New(db)

	tempDir := t.TempDir()

	sfs.AddWatch(tempDir, &File{})

	f1, _ := os.CreateTemp(tempDir, "*")

	time.Sleep(1 * time.Millisecond)

	df1 := File{}
	db.Find(&df1)

	t.Logf("%+v", df1)

	if f1.Name() != df1.Path {
		t.Error("paths don't match")
	}

}
