package event_test

import (
	"errors"
	"testing"
	"time"

	"github.com/roidmc/quotagate/internal/event"
)

var (
	testPayload = []byte(`{"type":"user.registered","subject":"user-123"}`)
	testSecret  = "test-secret-123"
	testNow     = time.Date(2025, 5, 16, 12, 0, 0, 0, time.UTC)
)

func allAlgorithms() []event.HashAlgorithm {
	return []event.HashAlgorithm{
		event.HashSHA256,
		event.HashSHA384,
		event.HashSHA512,
		event.HashSM3,
	}
}

func TestNewSigner(t *testing.T) {
	t.Run("valid algorithms", func(t *testing.T) {
		for _, alg := range allAlgorithms() {
			s, err := event.NewSigner(alg)
			if err != nil {
				t.Errorf("NewSigner(%s): unexpected error: %v", alg, err)
			}
			if s.Algorithm() != alg {
				t.Errorf("NewSigner(%s): expected Algorithm()=%s, got %s", alg, alg, s.Algorithm())
			}
		}
	})

	t.Run("unknown algorithm returns ErrUnknownAlgorithm", func(t *testing.T) {
		_, err := event.NewSigner(event.HashAlgorithm("unknown"))
		if !errors.Is(err, event.ErrUnknownAlgorithm) {
			t.Errorf("expected ErrUnknownAlgorithm, got: %v", err)
		}
	})

	t.Run("version tags are correct", func(t *testing.T) {
		expectedVersions := map[event.HashAlgorithm]string{
			event.HashSHA256: "v1",
			event.HashSHA384: "v384",
			event.HashSHA512: "v512",
			event.HashSM3:    "sm3",
		}
		for alg, wantVersion := range expectedVersions {
			s, _ := event.NewSigner(alg)
			if s.Version() != wantVersion {
				t.Errorf("NewSigner(%s): expected Version()=%s, got %s", alg, wantVersion, s.Version())
			}
		}
	})
}

func TestSignPayload(t *testing.T) {
	for _, alg := range allAlgorithms() {
		t.Run(string(alg), func(t *testing.T) {
			s, _ := event.NewSigner(alg)

			t.Run("returns valid SignatureResult", func(t *testing.T) {
				result, err := s.SignPayload(testPayload, testSecret, testNow)
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if result.Timestamp != testNow.Unix() {
					t.Errorf("expected timestamp %d, got %d", testNow.Unix(), result.Timestamp)
				}
				if result.Signature == "" {
					t.Error("expected non-empty signature")
				}
				if result.Header == "" {
					t.Error("expected non-empty header")
				}
			})

			t.Run("same input produces same signature", func(t *testing.T) {
				r1, _ := s.SignPayload(testPayload, testSecret, testNow)
				r2, _ := s.SignPayload(testPayload, testSecret, testNow)
				if r1.Signature != r2.Signature {
					t.Error("expected identical signatures for same input")
				}
				if r1.Header != r2.Header {
					t.Error("expected identical headers for same input")
				}
			})

			t.Run("different secret produces different signature", func(t *testing.T) {
				r1, _ := s.SignPayload(testPayload, testSecret, testNow)
				r2, _ := s.SignPayload(testPayload, "different-secret", testNow)
				if r1.Signature == r2.Signature {
					t.Error("expected different signatures for different secrets")
				}
			})

			t.Run("different payload produces different signature", func(t *testing.T) {
				r1, _ := s.SignPayload(testPayload, testSecret, testNow)
				r2, _ := s.SignPayload([]byte(`{"type":"user.login"}`), testSecret, testNow)
				if r1.Signature == r2.Signature {
					t.Error("expected different signatures for different payloads")
				}
			})

			t.Run("different timestamp produces different signature", func(t *testing.T) {
				r1, _ := s.SignPayload(testPayload, testSecret, testNow)
				r2, _ := s.SignPayload(testPayload, testSecret, testNow.Add(time.Second))
				if r1.Signature == r2.Signature {
					t.Error("expected different signatures for different timestamps")
				}
			})

			t.Run("empty secret returns ErrEmptySecret", func(t *testing.T) {
				_, err := s.SignPayload(testPayload, "", testNow)
				if !errors.Is(err, event.ErrEmptySecret) {
					t.Errorf("expected ErrEmptySecret, got: %v", err)
				}
			})
		})
	}
}

func TestSignPayloadHeaderFormat(t *testing.T) {
	expectedFormats := map[event.HashAlgorithm]string{
		event.HashSHA256: "t=1747396800,v1=",
		event.HashSHA384: "t=1747396800,v384=",
		event.HashSHA512: "t=1747396800,v512=",
		event.HashSM3:    "t=1747396800,sm3=",
	}

	for alg, prefix := range expectedFormats {
		t.Run(string(alg), func(t *testing.T) {
			s, _ := event.NewSigner(alg)
			result, _ := s.SignPayload(testPayload, testSecret, testNow)
			expected := prefix + result.Signature
			if result.Header != expected {
				t.Errorf("expected header %q, got %q", expected, result.Header)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	tolerance := event.DefaultTolerance

	for _, alg := range allAlgorithms() {
		t.Run(string(alg), func(t *testing.T) {
			s, _ := event.NewSigner(alg)

			t.Run("valid signature within tolerance", func(t *testing.T) {
				result, _ := s.SignPayload(testPayload, testSecret, time.Now())
				err := s.VerifySignature(testPayload, testSecret, result.Header, tolerance)
				if err != nil {
					t.Errorf("expected no error, got: %v", err)
				}
			})

			t.Run("wrong secret", func(t *testing.T) {
				result, _ := s.SignPayload(testPayload, testSecret, time.Now())
				err := s.VerifySignature(testPayload, "wrong-secret", result.Header, tolerance)
				if !errors.Is(err, event.ErrInvalidSignature) {
					t.Errorf("expected ErrInvalidSignature, got: %v", err)
				}
			})

			t.Run("tampered payload", func(t *testing.T) {
				result, _ := s.SignPayload(testPayload, testSecret, time.Now())
				err := s.VerifySignature([]byte(`{"type":"user.login"}`), testSecret, result.Header, tolerance)
				if !errors.Is(err, event.ErrInvalidSignature) {
					t.Errorf("expected ErrInvalidSignature, got: %v", err)
				}
			})

			t.Run("empty header", func(t *testing.T) {
				err := s.VerifySignature(testPayload, testSecret, "", tolerance)
				if !errors.Is(err, event.ErrInvalidSignature) {
					t.Errorf("expected ErrInvalidSignature, got: %v", err)
				}
			})

			t.Run("empty secret returns ErrEmptySecret", func(t *testing.T) {
				result, _ := s.SignPayload(testPayload, testSecret, time.Now())
				err := s.VerifySignature(testPayload, "", result.Header, tolerance)
				if !errors.Is(err, event.ErrEmptySecret) {
					t.Errorf("expected ErrEmptySecret, got: %v", err)
				}
			})

			t.Run("expired timestamp returns ErrTimestampExpired", func(t *testing.T) {
				oldTime := time.Now().Add(-10 * time.Minute)
				result, _ := s.SignPayload(testPayload, testSecret, oldTime)
				err := s.VerifySignature(testPayload, testSecret, result.Header, tolerance)
				if !errors.Is(err, event.ErrTimestampExpired) {
					t.Errorf("expected ErrTimestampExpired, got: %v", err)
				}
			})

			t.Run("future timestamp beyond tolerance returns ErrTimestampExpired", func(t *testing.T) {
				futureTime := time.Now().Add(10 * time.Minute)
				result, _ := s.SignPayload(testPayload, testSecret, futureTime)
				err := s.VerifySignature(testPayload, testSecret, result.Header, tolerance)
				if !errors.Is(err, event.ErrTimestampExpired) {
					t.Errorf("expected ErrTimestampExpired, got: %v", err)
				}
			})

			t.Run("timestamp at edge of tolerance is accepted", func(t *testing.T) {
				edgeTime := time.Now().Add(-4*time.Minute - 50*time.Second)
				result, _ := s.SignPayload(testPayload, testSecret, edgeTime)
				err := s.VerifySignature(testPayload, testSecret, result.Header, tolerance)
				if err != nil {
					t.Errorf("expected signature within tolerance to be accepted, got: %v", err)
				}
			})

			t.Run("custom tolerance of 1 hour accepts older timestamps", func(t *testing.T) {
				oldTime := time.Now().Add(-30 * time.Minute)
				result, _ := s.SignPayload(testPayload, testSecret, oldTime)
				err := s.VerifySignature(testPayload, testSecret, result.Header, 1*time.Hour)
				if err != nil {
					t.Errorf("expected signature within 1h tolerance to be accepted, got: %v", err)
				}
			})

			t.Run("custom tolerance of 1 hour rejects timestamps older than 1 hour", func(t *testing.T) {
				oldTime := time.Now().Add(-61 * time.Minute)
				result, _ := s.SignPayload(testPayload, testSecret, oldTime)
				err := s.VerifySignature(testPayload, testSecret, result.Header, 1*time.Hour)
				if !errors.Is(err, event.ErrTimestampExpired) {
					t.Errorf("expected ErrTimestampExpired, got: %v", err)
				}
			})
		})
	}
}

func TestCrossAlgorithmIsolation(t *testing.T) {
	t.Run("signature from one algorithm cannot be verified by another", func(t *testing.T) {
		algorithms := allAlgorithms()
		for i, alg1 := range algorithms {
			for j, alg2 := range algorithms {
				if i == j {
					continue
				}
				s1, _ := event.NewSigner(alg1)
				s2, _ := event.NewSigner(alg2)
				result, _ := s1.SignPayload(testPayload, testSecret, time.Now())
				err := s2.VerifySignature(testPayload, testSecret, result.Header, event.DefaultTolerance)
				if err == nil {
					t.Errorf("expected %s signer to reject %s signature, but it passed", alg2, alg1)
				}
			}
		}
	})

	t.Run("different algorithms produce different signatures for same input", func(t *testing.T) {
		results := make(map[event.HashAlgorithm]string)
		for _, alg := range allAlgorithms() {
			s, _ := event.NewSigner(alg)
			r, _ := s.SignPayload(testPayload, testSecret, testNow)
			results[alg] = r.Signature
		}
		seen := make(map[string]event.HashAlgorithm)
		for alg, sig := range results {
			if prev, exists := seen[sig]; exists {
				t.Errorf("algorithms %s and %s produced identical signatures", prev, alg)
			}
			seen[sig] = alg
		}
	})
}

func TestParseSignatureHeader(t *testing.T) {
	s, _ := event.NewSigner(event.HashSHA256)

	t.Run("malformed header without equals sign", func(t *testing.T) {
		err := s.VerifySignature(testPayload, testSecret, "garbage", event.DefaultTolerance)
		if !errors.Is(err, event.ErrMalformedHeader) {
			t.Errorf("expected ErrMalformedHeader, got: %v", err)
		}
	})

	t.Run("missing timestamp field", func(t *testing.T) {
		err := s.VerifySignature(testPayload, testSecret, "v1=abc123", event.DefaultTolerance)
		if !errors.Is(err, event.ErrMalformedHeader) {
			t.Errorf("expected ErrMalformedHeader, got: %v", err)
		}
	})

	t.Run("missing signature field", func(t *testing.T) {
		err := s.VerifySignature(testPayload, testSecret, "t=1747396800", event.DefaultTolerance)
		if !errors.Is(err, event.ErrMalformedHeader) {
			t.Errorf("expected ErrMalformedHeader, got: %v", err)
		}
	})

	t.Run("non-numeric timestamp", func(t *testing.T) {
		err := s.VerifySignature(testPayload, testSecret, "t=notanumber,v1=abc123", event.DefaultTolerance)
		if !errors.Is(err, event.ErrMalformedHeader) {
			t.Errorf("expected ErrMalformedHeader, got: %v", err)
		}
	})

	t.Run("extra fields are ignored", func(t *testing.T) {
		result, _ := s.SignPayload(testPayload, testSecret, time.Now())
		extraHeader := result.Header + ",extra=ignored"
		err := s.VerifySignature(testPayload, testSecret, extraHeader, event.DefaultTolerance)
		if err != nil {
			t.Errorf("expected extra fields to be ignored, got: %v", err)
		}
	})

	t.Run("SM3 signer looks for sm3 version tag", func(t *testing.T) {
		sm3Signer, _ := event.NewSigner(event.HashSM3)
		result, _ := sm3Signer.SignPayload(testPayload, testSecret, time.Now())
		err := sm3Signer.VerifySignature(testPayload, testSecret, result.Header, event.DefaultTolerance)
		if err != nil {
			t.Errorf("expected SM3 signer to verify its own signature, got: %v", err)
		}
	})

	t.Run("SHA256 signer cannot parse SM3 header", func(t *testing.T) {
		sm3Signer, _ := event.NewSigner(event.HashSM3)
		result, _ := sm3Signer.SignPayload(testPayload, testSecret, time.Now())
		err := s.VerifySignature(testPayload, testSecret, result.Header, event.DefaultTolerance)
		if !errors.Is(err, event.ErrMalformedHeader) {
			t.Errorf("expected ErrMalformedHeader when SHA256 signer parses SM3 header, got: %v", err)
		}
	})
}

func TestSignatureConstants(t *testing.T) {
	if event.SignatureHeader != "X-Webhook-Signature" {
		t.Errorf("expected X-Webhook-Signature, got %s", event.SignatureHeader)
	}
	if event.DefaultTolerance != 5*time.Minute {
		t.Errorf("expected 5m tolerance, got %v", event.DefaultTolerance)
	}
	if event.SignatureSep != "." {
		t.Errorf("expected ., got %s", event.SignatureSep)
	}
}

func TestDefaultSigner(t *testing.T) {
	t.Run("DefaultSigner uses SHA256", func(t *testing.T) {
		if event.DefaultSigner.Algorithm() != event.HashSHA256 {
			t.Errorf("expected DefaultSigner to use SHA256, got %s", event.DefaultSigner.Algorithm())
		}
		if event.DefaultSigner.Version() != "v1" {
			t.Errorf("expected DefaultSigner version v1, got %s", event.DefaultSigner.Version())
		}
	})

	t.Run("package-level SignPayload delegates to DefaultSigner", func(t *testing.T) {
		r1, err1 := event.SignPayload(testPayload, testSecret, testNow)
		r2, err2 := event.DefaultSigner.SignPayload(testPayload, testSecret, testNow)
		if err1 != err2 {
			t.Errorf("error mismatch: %v vs %v", err1, err2)
		}
		if r1.Signature != r2.Signature {
			t.Error("expected identical signatures from package function and DefaultSigner")
		}
		if r1.Header != r2.Header {
			t.Error("expected identical headers from package function and DefaultSigner")
		}
	})

	t.Run("package-level VerifySignature delegates to DefaultSigner", func(t *testing.T) {
		result, _ := event.SignPayload(testPayload, testSecret, time.Now())
		err := event.VerifySignature(testPayload, testSecret, result.Header, event.DefaultTolerance)
		if err != nil {
			t.Errorf("expected no error, got: %v", err)
		}
	})
}

func TestSignAndVerifyRoundTrip(t *testing.T) {
	for _, alg := range allAlgorithms() {
		t.Run(string(alg), func(t *testing.T) {
			s, _ := event.NewSigner(alg)

			t.Run("sign then verify succeeds", func(t *testing.T) {
				result, err := s.SignPayload(testPayload, testSecret, time.Now())
				if err != nil {
					t.Fatalf("SignPayload failed: %v", err)
				}
				err = s.VerifySignature(testPayload, testSecret, result.Header, event.DefaultTolerance)
				if err != nil {
					t.Errorf("VerifySignature failed: %v", err)
				}
			})

			t.Run("sign then verify with multiple payloads", func(t *testing.T) {
				payloads := [][]byte{
					[]byte(`{"type":"user.registered"}`),
					[]byte(`{"type":"user.login","ip":"1.2.3.4"}`),
					[]byte(`{"type":"order.created","id":"ord-999"}`),
					[]byte("plain text payload"),
					[]byte(""),
				}
				for _, p := range payloads {
					result, err := s.SignPayload(p, testSecret, time.Now())
					if err != nil {
						t.Fatalf("SignPayload failed: %v", err)
					}
					err = s.VerifySignature(p, testSecret, result.Header, event.DefaultTolerance)
					if err != nil {
						t.Errorf("VerifySignature failed for payload %q: %v", string(p), err)
					}
				}
			})

			t.Run("cross-signature rejection", func(t *testing.T) {
				payloadA := []byte(`{"type":"a"}`)
				payloadB := []byte(`{"type":"b"}`)
				resultA, _ := s.SignPayload(payloadA, testSecret, time.Now())
				err := s.VerifySignature(payloadB, testSecret, resultA.Header, event.DefaultTolerance)
				if err == nil {
					t.Error("expected verification to fail when payload differs from signed payload")
				}
			})
		})
	}
}
