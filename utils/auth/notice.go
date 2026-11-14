package auth

import (
	"fmt"
	"github.com/bedrock-tool/bedrocktool/locale"
	"io"

	"github.com/sandertv/gophertunnel/minecraft/auth"
)

// AntiCheatNotice is shown next to the Microsoft login code, in the GUI
// and the CLI (#348).
func AntiCheatNotice() string {
	return locale.Loc("anti_cheat_notice", nil)
}

// cliAuthCodeHandler prints AntiCheatNotice before the login code.
type cliAuthCodeHandler struct {
	auth.CliAuthCodeHandler
}

func newCliAuthCodeHandler(w io.Writer) cliAuthCodeHandler {
	return cliAuthCodeHandler{auth.CliAuthCodeHandler{Writer: w}}
}

// DisplayCode implements [auth.AuthCodeHandler].
func (c cliAuthCodeHandler) DisplayCode(verificationURI, userCode string) {
	fmt.Fprintf(c.Writer, "\n%s\n\n", AntiCheatNotice())
	c.CliAuthCodeHandler.DisplayCode(verificationURI, userCode)
}
