# Sqlfsync

Automatically syncronize your SQL database and filesystem. Built using [fsnotify](https://github.com/fsnotify/fsnotify) and [gorm](https://github.com/go-gorm/gorm).

## Installation

```
go get github.com/mattcarp12/sqlfsync
```

## Usage

```go
package main

import (
	"github.com/mattcarp12/sqlfsync"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"time"
)

type MyFile struct {
	ID        uint
	Path      string `sqlfsync:"path"`
	CreatedAt time.Time
}

var done chan bool

func main() {
	db, _ := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})

	sfs := sqlfsync.New(db)
	sfs.AddWatch("/my/files", &MyFile{})
	defer sfs.Close()

	done = make(chan bool)
	<-done
}
```

## Contributing
Pull requests are encouraged.

## License
