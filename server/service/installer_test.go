package service

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/fleetdm/fleet/v4/server/authz"
	"github.com/fleetdm/fleet/v4/server/contexts/ctxerr"
	"github.com/fleetdm/fleet/v4/server/fleet"
	"github.com/fleetdm/fleet/v4/server/mock"
	"github.com/fleetdm/fleet/v4/server/test"
	"github.com/stretchr/testify/require"
)

func setup(t *testing.T) (context.Context, *mock.Store, *mock.InstallerStore, fleet.Service) {
	t.Setenv("FLEET_DEMO", "1")
	ctx := test.UserContext(test.UserAdmin)
	ds := new(mock.Store)
	is := new(mock.InstallerStore)
	svc := newTestService(t, ds, nil, nil, &TestServerOpts{Is: is})
	ds.VerifyEnrollSecretFunc = func(ctx context.Context, enrollSecret string) (*fleet.EnrollSecret, error) {
		return &fleet.EnrollSecret{Secret: "xyz"}, nil

	}
	return ctx, ds, is, svc
}

func TestGetInstaller(t *testing.T) {
	t.Run("unauthorized access is not allowed", func(t *testing.T) {
		_, _, _, svc := setup(t)
		_, _, err := svc.GetInstaller(context.Background(), fleet.Installer{})
		require.Error(t, err)
		require.Contains(t, err.Error(), authz.ForbiddenErrorMessage)
	})

	t.Run("errors if store is not configured", func(t *testing.T) {
		ctx, ds, _, _ := setup(t)
		svc := newTestService(t, ds, nil, nil, &TestServerOpts{Is: nil})
		_, _, err := svc.GetInstaller(ctx, fleet.Installer{})
		require.Error(t, err)
		require.ErrorContains(t, err, "installer storage has not been configured")
	})

	t.Run("errors if the provided enroll secret cannot be found", func(t *testing.T) {
		ctx, ds, _, svc := setup(t)
		ds.VerifyEnrollSecretFunc = func(ctx context.Context, enrollSecret string) (*fleet.EnrollSecret, error) {
			return nil, notFoundError{}
		}
		_, _, err := svc.GetInstaller(ctx, fleet.Installer{})
		require.Error(t, err)
		require.ErrorAs(t, err, &notFoundError{})
		require.True(t, ds.VerifyEnrollSecretFuncInvoked)
	})

	t.Run("errors if there's a problem verifying the enroll secret", func(t *testing.T) {
		ctx, ds, _, svc := setup(t)
		ds.VerifyEnrollSecretFunc = func(ctx context.Context, enrollSecret string) (*fleet.EnrollSecret, error) {
			return nil, ctxerr.New(ctx, "test error")
		}
		_, _, err := svc.GetInstaller(ctx, fleet.Installer{})
		require.Error(t, err)
		require.ErrorContains(t, err, "test error")
		require.True(t, ds.VerifyEnrollSecretFuncInvoked)
	})

	t.Run("errors if there's a problem checking the blob storage", func(t *testing.T) {
		ctx, ds, is, svc := setup(t)
		is.GetFunc = func(ctx context.Context, installer fleet.Installer) (io.ReadCloser, int64, error) {
			return nil, int64(0), ctxerr.New(ctx, "test error")
		}
		_, _, err := svc.GetInstaller(ctx, fleet.Installer{})
		require.Error(t, err)
		require.ErrorContains(t, err, "test error")
		require.True(t, ds.VerifyEnrollSecretFuncInvoked)
		require.True(t, is.GetFuncInvoked)
	})

	t.Run("returns binary data with the installer", func(t *testing.T) {
		ctx, ds, is, svc := setup(t)
		is.GetFunc = func(ctx context.Context, installer fleet.Installer) (io.ReadCloser, int64, error) {
			str := "test"
			length := int64(len(str))
			reader := io.NopCloser(strings.NewReader(str))
			return reader, length, nil
		}
		blob, length, err := svc.GetInstaller(ctx, fleet.Installer{})
		require.NoError(t, err)
		body, err := io.ReadAll(blob)
		require.Equal(t, "test", string(body))
		require.EqualValues(t, length, len(body))
		require.NoError(t, err)
		require.True(t, ds.VerifyEnrollSecretFuncInvoked)
		require.True(t, is.GetFuncInvoked)
	})
}