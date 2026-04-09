package config

// rawFile is the TOML decode target for config.toml.
// Private struct — fields are populated from toml tags, then converted to Config in Load().
type rawFile struct {
	DefaultTeam string             `toml:"default_team"`
	User        UserConfig         `toml:"user"`
	Voice       VoiceConfig        `toml:"voice"`
	Sync        SyncConfig         `toml:"sync"`
	Shell       string             `toml:"shell"`
	Ask         AskConfig          `toml:"ask"`
	Flicknote   FlicknoteConfig    `toml:"flicknote"`
	Kubernetes  KubernetesConfig   `toml:"kubernetes"`
	Teams       map[string]rawTeam `toml:"teams"`
}

// rawTeam is the TOML decode target for a [teams.<name>] section.
type rawTeam struct {
	TeamPath             string              `toml:"team_path"`
	DataDir              string              `toml:"data_dir"`
	TaskRC               string              `toml:"taskrc"`
	TaskSyncURL          string              `toml:"task_sync_url"`
	ChatID               string              `toml:"chat_id"`
	Frontend             string              `toml:"frontend"`
	LifecycleAgent       string              `toml:"lifecycle_agent"`
	NotificationTokenEnv string              `toml:"notification_token_env"`
	DefaultRuntime       string              `toml:"default_runtime"`
	MergeMode            string              `toml:"merge_mode"`
	CommentSync          string              `toml:"comment_sync"`
	EmojiReactions       *bool               `toml:"emoji_reactions"`
	BreatheThreshold     *float64            `toml:"breathe_threshold"`
	User                 UserConfig          `toml:"user"`
	VoiceLanguage        string              `toml:"voice_language"`
	VoiceVocabulary      []string            `toml:"voice_vocabulary"`
	Agents               map[string]struct{} `toml:"agents"` // parsed but ignored (agent config lives in AGENTS.md)
	Matrix               *rawMatrix          `toml:"matrix"`
}

// rawMatrix mirrors MatrixTeamConfig for TOML decode.
type rawMatrix struct {
	Homeserver     string                    `toml:"homeserver"`
	NotifyRoom     string                    `toml:"notification_room"`
	NotifyTokenEnv string                    `toml:"notification_token_env"`
	HumanUserID    string                    `toml:"human_user_id"`
	Agents         map[string]rawMatrixAgent `toml:"agents"`
}

type rawMatrixAgent struct {
	AccessTokenEnv string `toml:"access_token_env"`
	RoomID         string `toml:"room_id"`
}
