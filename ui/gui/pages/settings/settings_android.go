package settings

import (
	"strconv"

	"gioui.org/layout"
	"gioui.org/widget"
	"gioui.org/widget/material"
	"github.com/bedrock-tool/bedrocktool/ui/gui/android"
	"github.com/bedrock-tool/bedrocktool/ui/gui/popups"
)

// settingsPlatform is the start screen's Android-only part: a card that
// opens the captures and saved worlds, which a phone's file managers can't
// see in the app's folder.
type settingsPlatform struct {
	files widget.Clickable
}

// platformCards go under the mode cards on the start screen.
func (p *Page) platformCards(th *material.Theme) []layout.FlexChild {
	return []layout.FlexChild{
		layout.Rigid(func(gtx C) D {
			if p.platform.files.Clicked(gtx) {
				p.g.ShowPopup(android.NewFilesPopup(p.g, p.replayCapture))
			}
			return layout.Inset{Top: 10, Bottom: 8}.Layout(gtx, func(gtx C) D {
				return modeCard(gtx, th, &p.platform.files, android.T("android_files_title", nil), android.T("android_files_card", nil))
			})
		}),
	}
}

// replayCapture saves the worlds in a capture with the worlds mode's
// settings, merged into one world if merge. The address and the merge
// option go back to what they were after, so the next session isn't a
// replay by surprise.
func (p *Page) replayCapture(path string, merge bool) {
	s, ok := p.settings["worlds"]
	if !ok {
		return
	}
	prevAddress := AddressInput.Text()
	prevMerge, hadMerge := s.Values()["merge-dimensions"]
	defer func() {
		AddressInput.SetText(prevAddress)
		if hadMerge {
			s.Restore(map[string]string{"merge-dimensions": prevMerge})
		}
	}()

	AddressInput.SetText(path)
	s.Restore(map[string]string{"merge-dimensions": strconv.FormatBool(merge)})
	settings, err := s.Apply()
	if err != nil {
		p.g.ShowPopup(popups.NewErrorPopup(p.g, err, false, nil))
		return
	}
	p.g.StartSubcommand("worlds", settings)
}
