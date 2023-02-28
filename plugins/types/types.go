package types

type JobData struct {
	Target    string
	ChannelID string
	Amount    int64
}

type Error string

func (e Error) Error() string { return string(e) }

const ErrCouldNotParseJobData = Error("could not parse job data")
const ErrInvalidArguments = Error("invalid arguments")
