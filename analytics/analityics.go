package analytics

// TrackEvent uses the default analytics client to track an event
func TrackEvent(event string, properties map[string]interface{}) error {
	return nil
}

// IdentifyUser uses the default analytics client to identify the user
func IdentifyUser(userConfig interface{}) error {
	return nil

}
