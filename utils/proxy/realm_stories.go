package proxy

import (
	"context"
	"errors"
	"fmt"

	"github.com/bedrock-tool/bedrocktool/ui/messages"
	"github.com/sandertv/gophertunnel/minecraft/realms"
	"github.com/sirupsen/logrus"
)

// answerRealmStories handles a Realm refusing the join because the
// account never answered its Stories timeline prompt (which the game asks
// on the first join): it asks the player for the answer, saves it, and
// says whether to try joining again.
func answerRealmStories(ctx context.Context, realm *realms.Realm) bool {
	settings, err := realm.StorySettings(ctx)
	if err != nil {
		logrus.Debugf("realm stories settings: %s", err)
		return false
	}
	if settings.PlayerOptIn != realms.StoryOptInNone && settings.PlayerOptIn != "" {
		return false // answered already: the refusal is something else
	}

	logrus.Infof("%s: the Realm's Stories prompt was never answered, so it won't let you join yet", realm.Name)
	switch messages.AskStoriesPrompt(ctx, realm.Name) {
	case messages.StoriesOptIn:
		settings.PlayerOptIn = realms.StoryOptInOptIn
	case messages.StoriesOptOut:
		settings.PlayerOptIn = realms.StoryOptInOptOut
	default:
		return false
	}
	if err := realm.UpdateStorySettings(ctx, settings); err != nil {
		logrus.Errorf("saving the Stories answer: %s", err)
		return false
	}
	logrus.Infof("Stories answer saved (%s), joining again", settings.PlayerOptIn)
	return true
}

// realmJoinError says why a Realm couldn't be joined in words a player
// can act on.
func realmJoinError(realm *realms.Realm, err error) error {
	switch {
	case realm.Expired:
		return fmt.Errorf("%s has expired (its owner's subscription ran out), so it can't be joined: %w", realm.Name, err)
	case errors.Is(err, realms.ErrPlayerNotInRealm):
		return fmt.Errorf("%s refused the join. If you've never joined it from Minecraft, join it there once and answer its prompts, then try again: %w", realm.Name, err)
	}
	return err
}
