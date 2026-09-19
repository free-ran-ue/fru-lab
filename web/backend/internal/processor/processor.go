package processor

import (
	"backend/logger"
	"time"
)

type ProcessorIE struct {
	Username string
	Password string

	JwtSecret    string
	JwtExpiresIn time.Duration

	*logger.BackendLogger
}

type Processor struct {
	username string
	password string

	jwtSecret    string
	jwtExpiresIn time.Duration

	*logger.BackendLogger
}

func NewProcessor(ie *ProcessorIE) *Processor {
	return &Processor{
		username: ie.Username,
		password: ie.Password,

		jwtSecret:    ie.JwtSecret,
		jwtExpiresIn: ie.JwtExpiresIn,

		BackendLogger: ie.BackendLogger,
	}
}
