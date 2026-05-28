package secrets

import (
	"github.com/charmbracelet/huh"
)

func newPasswordForm(title string, dest *string) *huh.Form {
	return huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title(title).
				EchoMode(huh.EchoModePassword).
				Value(dest),
		),
	)
}
