package auth

import (
	"fmt"
	"io"

	"github.com/sandertv/gophertunnel/minecraft/auth"
)

// AntiCheatNotice is shown next to the Microsoft login code, in the GUI
// and the CLI (#348).
const AntiCheatNotice = "A note before you sign in: on servers with strict anti-cheat (Cubecraft and The Hive are known cases), " +
	"using bedrocktool can in rare cases get the account flagged or banned. " +
	"It joins through gophertunnel, which doesn't move or send input exactly like the real game, so some anti-cheats can tell it apart. " +
	"For those servers use an alt account; Realms, your own worlds and most servers are fine."

// cliAuthCodeHandler prints AntiCheatNotice before the login code.
type cliAuthCodeHandler struct {
	auth.CliAuthCodeHandler
}

func newCliAuthCodeHandler(w io.Writer) cliAuthCodeHandler {
	return cliAuthCodeHandler{auth.CliAuthCodeHandler{Writer: w}}
}

// DisplayCode implements [auth.AuthCodeHandler].
func (c cliAuthCodeHandler) DisplayCode(verificationURI, userCode string) {
	fmt.Fprintf(c.Writer, "\n%s\n\n", AntiCheatNotice)
	c.CliAuthCodeHandler.DisplayCode(verificationURI, userCode)
}
