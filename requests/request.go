package requests

type Request interface {
	Validate() (bool, string, string)
}
