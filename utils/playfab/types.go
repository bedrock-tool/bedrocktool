package playfab

import "time"

type Response[T any] struct {
	Code   int    `json:"code"`
	Status string `json:"string"`
	Data   *T     `json:"data"`
}

type PlayfabError struct {
	StatusCode   int `json:"-"`
	Status       string
	ErrorCode    int    `json:"errorCode"`
	ErrorMessage string `json:"errorMessage"`
}

func (p *PlayfabError) Error() string {
	return p.ErrorMessage
}

type Entity struct {
	ID         string `json:"Id"`
	Type       string
	TypeString string
}

type EntityToken struct {
	EntityToken     string    `json:"EntityToken"`
	TokenExpiration time.Time `json:"TokenExpiration"`
	Entity          *Entity   `json:"Entity"`
}

type infoRequestParameters struct {
	CharacterInventories bool `json:"GetCharacterInventories"`
	CharacterList        bool `json:"GetCharacterList"`
	PlayerProfile        bool `json:"GetPlayerProfile"`
	PlayerStatistics     bool `json:"GetPlayerStatistics"`
	TitleData            bool `json:"GetTitleData"`
	UserAccountInfo      bool `json:"GetUserAccountInfo"`
	UserData             bool `json:"GetUserData"`
	UserInventory        bool `json:"GetUserInventory"`
	UserReadOnlyData     bool `json:"GetUserReadOnlyData"`
	UserVirtualCurrency  bool `json:"GetUserVirtualCurrency"`
	PlayerStatisticNames any  `json:"PlayerStatisticNames"`
	ProfileConstraints   any  `json:"ProfileConstraints"`
	TitleDataKeys        any  `json:"TitleDataKeys"`
	UserDataKeys         any  `json:"UserDataKeys"`
	UserReadOnlyDataKeys any  `json:"UserReadOnlyDataKeys"`
}

type xboxLoginRequest struct {
	CreateAccount         bool                  `json:"CreateAccount"`
	EncryptedRequest      any                   `json:"EncryptedRequest"`
	InfoRequestParameters infoRequestParameters `json:"InfoRequestParameters"`
	PlayerSecret          any                   `json:"PlayerSecret"`
	TitleID               string                `json:"TitleId"`
	XboxToken             string                `json:"XboxToken"`
}

type loginResponse struct {
	SessionTicket   string `json:"SessionTicket"`
	PlayFabID       string `json:"PlayFabId"`
	NewlyCreated    bool   `json:"NewlyCreated"`
	SettingsForUser struct {
		NeedsAttribution bool `json:"NeedsAttribution"`
		GatherDeviceInfo bool `json:"GatherDeviceInfo"`
		GatherFocusInfo  bool `json:"GatherFocusInfo"`
	} `json:"SettingsForUser"`
	LastLoginTime     time.Time `json:"LastLoginTime"`
	InfoResultPayload struct {
		AccountInfo struct {
			PlayFabID string    `json:"PlayFabId"`
			Created   time.Time `json:"Created"`
			TitleInfo struct {
				DisplayName        string    `json:"DisplayName"`
				Origination        string    `json:"Origination"`
				Created            time.Time `json:"Created"`
				LastLogin          time.Time `json:"LastLogin"`
				FirstLogin         time.Time `json:"FirstLogin"`
				IsBanned           bool      `json:"isBanned"`
				TitlePlayerAccount struct {
					ID         string `json:"Id"`
					Type       string `json:"Type"`
					TypeString string `json:"TypeString"`
				} `json:"TitlePlayerAccount"`
			} `json:"TitleInfo"`
			PrivateInfo struct {
			} `json:"PrivateInfo"`
			XboxInfo struct {
				XboxUserID      string `json:"XboxUserId"`
				XboxUserSandbox string `json:"XboxUserSandbox"`
			} `json:"XboxInfo"`
		} `json:"AccountInfo"`
		UserInventory           []any `json:"UserInventory"`
		UserDataVersion         int   `json:"UserDataVersion"`
		UserReadOnlyDataVersion int   `json:"UserReadOnlyDataVersion"`
		CharacterInventories    []any `json:"CharacterInventories"`
		PlayerProfile           struct {
			PublisherID string `json:"PublisherId"`
			TitleID     string `json:"TitleId"`
			PlayerID    string `json:"PlayerId"`
			DisplayName string `json:"DisplayName"`
		} `json:"PlayerProfile"`
	} `json:"InfoResultPayload"`
	EntityToken         EntityToken `json:"EntityToken"`
	TreatmentAssignment struct {
		Variants  []any `json:"Variants"`
		Variables []any `json:"Variables"`
	} `json:"TreatmentAssignment"`
}

type entityTokenRequest struct {
	Entity *Entity `json:"Entity"`
}

type mcTokenDevice struct {
	ApplicationType    string   `json:"applicationType"`
	Capabilities       []string `json:"capabilities"`
	GameVersion        string   `json:"gameVersion"`
	ID                 string   `json:"id"`
	Memory             string   `json:"memory"`
	Platform           string   `json:"platform"`
	PlayFabTitleID     string   `json:"playFabTitleId"`
	StorePlatform      string   `json:"storePlatform"`
	TreatmentOverrides any      `json:"treatmentOverrides"`
	Type               string   `json:"type"`
}

type mcTokenUser struct {
	Language     string `json:"language"`
	LanguageCode string `json:"languageCode"`
	RegionCode   string `json:"regionCode"`
	Token        string `json:"token"`
	TokenType    string `json:"tokenType"`
}
type mcTokenRequest struct {
	Device mcTokenDevice `json:"device"`
	User   mcTokenUser   `json:"user"`
}

type MCToken struct {
	AuthorizationHeader string    `json:"authorizationHeader"`
	ValidUntil          time.Time `json:"validUntil"`
	Treatments          []string  `json:"treatments"`
	Configurations      struct {
		Minecraft struct {
			ID         string         `json:"id"`
			Parameters map[string]any `json:"parameters"`
		} `json:"minecraft"`
	} `json:"configurations"`
}

type mcTokenResponse struct {
	Result MCToken `json:"result"`
}
