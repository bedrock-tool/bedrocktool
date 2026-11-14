package proxy

import (
	"context"
	"errors"
	"fmt"
	"github.com/bedrock-tool/bedrocktool/locale"

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

	logrus.Info(locale.Loc("stories_unanswered", locale.Strmap{"Realm": realm.Name}))
	switch messages.AskStoriesPrompt(ctx, realm.Name) {
	case messages.StoriesOptIn:
		settings.PlayerOptIn = realms.StoryOptInOptIn
	case messages.StoriesOptOut:
		settings.PlayerOptIn = realms.StoryOptInOptOut
	default:
		return false
	}
	if err := realm.UpdateStorySettings(ctx, settings); err != nil {
		logrus.Error(locale.Loc("stories_save_failed", locale.Strmap{"Err": err}))
		return false
	}
	logrus.Info(locale.Loc("stories_saved", locale.Strmap{"Answer": settings.PlayerOptIn}))
	return true
}

// realmJoinError says why a Realm couldn't be joined in words a player
// can act on.
func realmJoinError(realm *realms.Realm, err error) error {
	switch {
	case realm.Expired:
		return fmt.Errorf("%s: %w", locale.Loc("realm_expired_error", locale.Strmap{"Realm": realm.Name}), err)
	case errors.Is(err, realms.ErrPlayerNotInRealm):
		return fmt.Errorf("%s: %w", locale.Loc("realm_refused_join", locale.Strmap{"Realm": realm.Name}), err)
	}
	return err
}
