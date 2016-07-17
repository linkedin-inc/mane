package service

type Saver interface {
	Save(destination string, items []interface{}) error
}

var saver Saver

func RegsiterSaver(s Saver) {
	saver = s
}
