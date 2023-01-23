package locale

import (
	"embed"
	"fmt"

	"github.com/cloudfoundry-attic/jibber_jabber"
	"github.com/nicksnyder/go-i18n/v2/i18n"
	"github.com/sirupsen/logrus"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
)

type Strmap map[string]interface{}

//go:embed *.yaml
var localesFS embed.FS
var lang *i18n.Localizer

func load_language(bundle *i18n.Bundle, tag language.Tag) error {
	logrus.Infof("Using Language %s", tag.String())
	_, err := bundle.LoadMessageFileFS(localesFS, fmt.Sprintf("%s.yaml", tag.String()))
	return err
}

func init() {
	var defaultTag language.Tag = language.English
	var err error

	// get default language
	var languageName string
	languageName, err = jibber_jabber.DetectLanguage()
	if err == nil {
		defaultTag, err = language.Parse(languageName)
		if err != nil {
			logrus.Warn("failed to parse language name")
		}
	}

	bundle := i18n.NewBundle(defaultTag)
	bundle.RegisterUnmarshalFunc("yaml", yaml.Unmarshal)

	if defaultTag != language.English {
		err = load_language(bundle, language.English)
		if err != nil {
			panic("failed to load english language")
		}
	}

	err = load_language(bundle, defaultTag)
	if err != nil {
		logrus.Warnf("Couldnt load Language %s", languageName)
	}

	lang = i18n.NewLocalizer(bundle)
}

func Locm(id string, tmpl Strmap, count int) string {
	return Loc(id, tmpl)
}

func Loc(id string, tmpl Strmap) string {
	s, err := lang.Localize(&i18n.LocalizeConfig{
		MessageID:    id,
		TemplateData: tmpl,
	})
	if err != nil {
		return fmt.Sprintf("failed to translate! %s", id)
	}
	return s
}
