package monerotipbot

var (
	// ProgramName is our program name string
	ProgramName string

	// Version is the version of the bot
	Version   = "v1.1.0"
	buildTime = "03-10-2019 17:51"
)

func init() {
	// programName = filepath.Base(os.Args[0])
	ProgramName = "monerotipbot"
}
