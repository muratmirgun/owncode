package main

import (
	"github.com/muratmirgun/owncode/cmd"
	"github.com/muratmirgun/owncode/internal/logging"
)

func main() {
	defer logging.RecoverPanic("main", func() {
		logging.ErrorPersist("Application terminated due to unhandled panic")
	})

	cmd.Execute()
}
