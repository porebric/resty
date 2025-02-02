package requests

type Request interface {
	Validate() (bool, string, string)
	Methods() []string
	Path() (string, bool)
	String() string
}
