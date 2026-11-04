package main

import (
	"bikers.kz/backend/internal/config"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

// Key generation is an operator command: secret material goes only to a new,
// owner-readable file. It never appears in stdout, logs or a process argument.
func runPushKeys(args []string) error {
	flags := flag.NewFlagSet("push-keys", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	output := flags.String("output", "", "new private environment file")
	subject := flags.String("subject", "", "operator HTTPS contact URL or mailto address")
	if err := flags.Parse(args); err != nil || flags.NArg() != 0 || *output == "" || *subject == "" {
		return errors.New("usage: api push-keys --output NEW_ENV_FILE --subject CONTACT_URI")
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return errors.New("could not generate Web Push keys")
	}
	public := base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes())
	private := base64.RawURLEncoding.EncodeToString(key.Bytes())
	if err = config.ValidatePush(public, private, *subject); err != nil {
		return err
	}
	file, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("could not create new Web Push environment file; existing files are never replaced")
	}
	// Compose treats single-quoted .env values literally, so a valid contact
	// URI containing dollar signs cannot interpolate another environment value.
	contact := strings.ReplaceAll(*subject, "'", "\\'")
	_, writeErr := fmt.Fprintf(file, "WEB_PUSH_PUBLIC_KEY=%s\nWEB_PUSH_PRIVATE_KEY=%s\nWEB_PUSH_SUBJECT='%s'\n", public, private, contact)
	closeErr := file.Close()
	if writeErr != nil || closeErr != nil {
		_ = os.Remove(*output)
		return errors.New("could not write Web Push environment file")
	}
	return nil
}
