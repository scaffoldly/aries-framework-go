//go:build ursa
// +build ursa

/*
 Copyright Avast Software. All Rights Reserved.

 SPDX-License-Identifier: Apache-2.0
*/

package localkms

import (
	"testing"

	"github.com/google/tink/go/keyset"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/aries-framework-go/pkg/kms"
	mockstorage "github.com/hyperledger/aries-framework-go/pkg/mock/storage"
)

func TestLocalKMS_Ursa_Success(t *testing.T) {
	// create a real (not mocked) master key and secret lock to test the KMS end to end
	sl := createMasterKeyAndSecretLock(t)

	storeDB := make(map[string]mockstorage.DBEntry)
	// test New()
	mockStorageProvider := mockstorage.NewCustomMockStoreProvider(
		&mockstorage.MockStore{
			Store: storeDB,
		})

	kmsStore, err := kms.NewAriesProviderWrapper(mockStorageProvider)
	require.NoError(t, err)

	kmsService, err := New(testMasterKeyURI, &mockProvider{
		storage:    kmsStore,
		secretLock: sl,
	})
	require.NoError(t, err)
	require.NotEmpty(t, kmsService)

	keyTemplates := []kms.KeyType{
		kms.CLCredDefType,
		kms.CLMasterSecretType,
	}

	opts := make(map[kms.KeyType]kms.KeyOpts)
	opts[kms.CLCredDefType] = kms.WithAttrs([]string{"attr1", "attr2"})

	for _, v := range keyTemplates {
		// test Create() a new key
		keyID, newKeyHandle, e := kmsService.Create(v, opts[v])
		require.NoError(t, e, "failed on template %v", v)
		require.NotEmpty(t, newKeyHandle)
		require.NotEmpty(t, keyID)

		ks, ok := storeDB[keyID]
		require.True(t, ok)
		require.NotEmpty(t, ks)

		// get key handle primitives
		newKHPrimitives, e := newKeyHandle.(*keyset.Handle).Primitives()
		require.NoError(t, e)
		require.NotEmpty(t, newKHPrimitives)

		// test Get() an existing keyhandle (it should match newKeyHandle above)
		loadedKeyHandle, e := kmsService.Get(keyID)
		require.NoError(t, e)
		require.NotEmpty(t, loadedKeyHandle)

		readKHPrimitives, e := loadedKeyHandle.(*keyset.Handle).Primitives()
		require.NoError(t, e)
		require.NotEmpty(t, newKHPrimitives)

		require.Equal(t, len(newKHPrimitives.Entries), len(readKHPrimitives.Entries))

		// finally test Rotate()
		// with unsupported key type - should fail
		newKeyID, rotatedKeyHandle, e := kmsService.Rotate("unsupported", keyID)
		require.Error(t, e)
		require.Empty(t, rotatedKeyHandle)
		require.Empty(t, newKeyID)

		// with valid key type - should succeed
		newKeyID, rotatedKeyHandle, e = kmsService.Rotate(v, keyID, opts[v])
		require.NoError(t, e)
		require.NotEmpty(t, rotatedKeyHandle)
		require.NotEqual(t, newKeyID, keyID)

		rotatedKHPrimitives, e := loadedKeyHandle.(*keyset.Handle).Primitives()
		require.NoError(t, e)
		require.NotEmpty(t, newKHPrimitives)
		require.Equal(t, len(newKHPrimitives.Entries), len(rotatedKHPrimitives.Entries))
		require.Equal(t, len(readKHPrimitives.Entries), len(rotatedKHPrimitives.Entries))

		if v == kms.CLCredDefType {
			pubKeyBytes, kt, e := kmsService.ExportPubKeyBytes(keyID)
			require.Errorf(t, e, "KeyID has been rotated. An error must be returned")
			require.Empty(t, pubKeyBytes)
			require.Empty(t, kt)

			pubKeyBytes, kt, e = kmsService.ExportPubKeyBytes(newKeyID)
			require.NoError(t, e)
			require.NotEmpty(t, pubKeyBytes)
			require.Equal(t, v, kt)

			kh, e := kmsService.PubKeyBytesToHandle(pubKeyBytes, v)
			require.NoError(t, e)
			require.NotEmpty(t, kh)

			// test create and export key in one function
			_, _, e = kmsService.CreateAndExportPubKeyBytes(v, opts[v])
			require.NoError(t, e)
		}
	}
}
