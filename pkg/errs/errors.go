package errs

import (
	"github.com/emersion/go-smtp"
)

var (
	ErrInvalidEmail = &smtp.SMTPError{
		Code:         501,
		EnhancedCode: smtp.EnhancedCode{5, 1, 7},
		Message:      "Invalid email address",
	}

	ErrFromDisallowed = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 1, 8},
		Message:      "Sender address is not allowed",
	}

	ErrToDisallowed = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 1, 1},
		Message:      "Recipient address is not allowed",
	}

	ErrTooManyRecipients = &smtp.SMTPError{
		Code:         452,
		EnhancedCode: smtp.EnhancedCode{4, 5, 3},
		Message:      "Too many recipients",
	}

	ErrSourceIPDisallowed = &smtp.SMTPError{
		Code:         550,
		EnhancedCode: smtp.EnhancedCode{5, 7, 1},
		Message:      "Access denied by IP policy",
	}

	ErrSourceIPInvalid = &smtp.SMTPError{
		Code:         421,
		EnhancedCode: smtp.EnhancedCode{4, 4, 0},
		Message:      "Source IP address is invalid",
	}
)
