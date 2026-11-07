package dialer

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"

	"veil/internal/transport/netpin"
)

const maxCallResponse = 1 << 20

func (d *Dialer) Call(ctx context.Context, subpath string, in, out any) error {
	targets, err := d.Profile.Targets()
	if err != nil {
		return err
	}
	authKey, err := base64.StdEncoding.DecodeString(d.Profile.AuthKey)
	if err != nil {
		return fmt.Errorf("auth key: %w", err)
	}
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	dial := d.Dial
	if dial == nil {
		dial = (&net.Dialer{Control: netpin.Control}).DialContext
	}
	var errs []error
	for _, t := range targets {
		raw, err := dial(ctx, "tcp", t.Addr.String())
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", t.Family, err))
			continue
		}
		stop := context.AfterFunc(ctx, func() { _ = raw.Close() })
		err = d.callOn(raw, t.Family, authKey, subpath, body, out)
		stop()
		_ = raw.Close()
		if err == nil {
			return nil
		}
		var apiErr *CallError
		if errors.As(err, &apiErr) {
			return err
		}
		errs = append(errs, fmt.Errorf("%s: %w", t.Family, err))
	}
	return errors.Join(errs...)
}

type CallError struct {
	Status  int
	Message string
}

func (e *CallError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	return fmt.Sprintf("сервер ответил HTTP %d", e.Status)
}

func (d *Dialer) callOn(raw net.Conn, family string, authKey []byte, subpath string, body []byte, out any) error {
	_, cc, err := d.h2Client(raw)
	if err != nil {
		return err
	}
	defer cc.Close()
	targets, _ := d.Profile.Targets()
	t := targets[0]
	for _, x := range targets {
		if x.Family == family {
			t = x
		}
	}
	req, err := d.tunnelRequest(t, authKey, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.URL.Path += subpath
	req.Header.Set("Content-Type", "application/json")
	resp, err := cc.RoundTrip(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxCallResponse))
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error string `json:"error"`
		}
		_ = json.Unmarshal(data, &e)
		return &CallError{Status: resp.StatusCode, Message: e.Error}
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(data, out)
}

const probeRoundTrips = 3

func (d *Dialer) Probe(ctx context.Context, subpath string, in any) (time.Duration, error) {
	start := time.Now()
	if err := d.Call(ctx, subpath, in, nil); err != nil {
		return 0, err
	}
	return time.Since(start) / probeRoundTrips, nil
}
