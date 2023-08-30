//go:build !js

package locale

import "github.com/cloudfoundry/jibber_jabber"

func getLanguageName() string {
	languageName, _ := jibber_jabber.DetectLanguage()
	return languageName
}
