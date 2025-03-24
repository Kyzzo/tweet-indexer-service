package httperror

const (
	InternalServerError = "Internal server error :/"
	NotFoundError       = "Not found :("
)

type RestError struct {
	Msg      string
	Httpcode int
	ErrItems []RestItemError
}

type RestItemError struct {
	msg  string
	code int
}

func NewError(msg string, httpcode int) RestError {
	return RestError{
		Msg:      msg,
		Httpcode: httpcode,
		ErrItems: []RestItemError{},
	}
}

func NewInternalServerError() RestError {
	return RestError{
		Msg:      InternalServerError,
		Httpcode: 500,
		ErrItems: []RestItemError{},
	}
}

func NewNotFoundError() RestError {
	return RestError{
		Msg:      NotFoundError,
		Httpcode: 500,
		ErrItems: []RestItemError{},
	}
}

func NewNotFoundErrorWithMsg(msg string) RestError {
	return RestError{
		Msg:      msg,
		Httpcode: 404,
		ErrItems: []RestItemError{},
	}
}

func NewInternalServerErrorWithMsg(msg string) RestError {
	return RestError{
		Msg:      msg,
		Httpcode: 500,
		ErrItems: []RestItemError{},
	}
}

func NewErrorWithItems(msg string, httpcode int, errItems []RestItemError) RestError {
	return RestError{
		Msg:      msg,
		Httpcode: httpcode,
		ErrItems: errItems,
	}
}

func (r RestError) Error() string {
	return r.Msg
}
