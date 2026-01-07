package subcommands

import (
	"context"
	"fmt"
	"os"

	"github.com/bedrock-tool/bedrocktool/utils/auth"
	"github.com/bedrock-tool/bedrocktool/utils/commands"
)

type ImportTokenSettings struct {
	File string `opt:"File" flag:"file" desc:"Path to token JSON file (use '-' for stdin)"`
	Name string `opt:"Name" flag:"name" desc:"Optional account name to store token under"`
}

type ImportTokenCMD struct{}

func (ImportTokenCMD) Name() string { return "auth-import" }
func (ImportTokenCMD) Description() string { return "Import an OAuth token JSON file to be used by the CLI" }
func (ImportTokenCMD) Settings() any { return new(ImportTokenSettings) }

func (ImportTokenCMD) Run(ctx context.Context, settings any) error {
	s := settings.(*ImportTokenSettings)
	if s.File == "" || s.File == "-" {
		// Read from stdin
		f, err := os.CreateTemp("", "token-*.json")
		if err != nil {
			return err
		}
		defer func() { _ = os.Remove(f.Name()) }()
		_, err = f.ReadFrom(os.Stdin)
		if err != nil {
			return err
		}
		_ = f.Close()
		s.File = f.Name()
	}

	if _, err := os.Stat(s.File); err != nil {
		return fmt.Errorf("read token file: %w", err)
	}
	if err := auth.ImportTokenFile(s.File, s.Name); err != nil {
		return fmt.Errorf("import token: %w", err)
	}
	fmt.Println("Token imported successfully")
	return nil
}

func init() {
	commands.RegisterCommand(&ImportTokenCMD{})
}
