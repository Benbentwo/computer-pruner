package preferences

// AppPreferences represents the application preferences and settings
type AppPreferences struct {
	Theme                  string   `json:"theme"`
	DefaultDeleteBehavior  string   `json:"defaultDeleteBehavior"`
	ScanDepthLimit         int      `json:"scanDepthLimit"`
	ExclusionPaths         []string `json:"exclusionPaths"`
}

// PreferencesService manages application preferences
type PreferencesService struct {
}

// NewPreferencesService creates a new instance of PreferencesService
func NewPreferencesService() *PreferencesService {
	return &PreferencesService{}
}

// GetPreferences returns the current application preferences with sensible defaults
func (p *PreferencesService) GetPreferences() *AppPreferences {
	return &AppPreferences{
		Theme:                 "auto",
		DefaultDeleteBehavior: "trash",
		ScanDepthLimit:        8,
		ExclusionPaths: []string{
			"/System",
			"/Library",
			"/Applications",
		},
	}
}

// SetPreferences updates the application preferences
func (p *PreferencesService) SetPreferences(prefs AppPreferences) error {
	// TODO: Implement actual preferences persistence
	return nil
}
