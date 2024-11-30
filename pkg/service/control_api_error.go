package service

type ControlAPIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (err *ControlAPIError) Error() string {
	return err.Message
}
