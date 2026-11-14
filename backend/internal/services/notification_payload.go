package services

import (
	"encoding/json"
	"errors"

	"firebase.google.com/go/v4/messaging"
)

const maxFCMRequestBytes = 4096

// limitPushPreview leaves history and routing data intact. Count the complete
// single-device request, including escaped JSON, token and Web Push options,
// rather than assuming that a character count fits FCM's 4096-byte payload.
func limitPushPreview(message *messaging.MulticastMessage) error {
	fits := func() (bool, error) {
		request := struct {
			Message *messaging.Message `json:"message"`
		}{Message: &messaging.Message{
			Token: message.Tokens[0], Notification: message.Notification,
			Data: message.Data, Webpush: message.Webpush,
		}}
		encoded, err := json.Marshal(request)
		return len(encoded) <= maxFCMRequestBytes, err
	}
	if ok, err := fits(); ok || err != nil {
		return err
	}
	// Keep the title when possible. If metadata plus title is too large, trim
	// that too. Routing data is never removed to make a preview fit.
	for _, field := range []*string{&message.Notification.Body, &message.Notification.Title} {
		runes := []rune(*field)
		*field = ""
		ok, err := fits()
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		*field = "…"
		if ok, err := fits(); err != nil {
			return err
		} else if !ok {
			*field = ""
			return nil
		}
		low, high := 0, len(runes)
		for low < high {
			mid := low + (high-low+1)/2
			*field = string(runes[:mid]) + "…"
			ok, err := fits()
			if err != nil {
				return err
			}
			if ok {
				low = mid
			} else {
				high = mid - 1
			}
		}
		*field = string(runes[:low]) + "…"
		return nil
	}
	return errors.New("push routing metadata exceeds the provider payload limit")
}
