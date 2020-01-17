/*
Copyright SecureKey Technologies Inc. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package route

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/hyperledger/aries-framework-go/pkg/didcomm/common/service"
	"github.com/hyperledger/aries-framework-go/pkg/doc/did"
	"github.com/hyperledger/aries-framework-go/pkg/framework/aries/api/vdri"
	mockdispatcher "github.com/hyperledger/aries-framework-go/pkg/internal/mock/didcomm/dispatcher"
	mockdiddoc "github.com/hyperledger/aries-framework-go/pkg/internal/mock/diddoc"
	mockkms "github.com/hyperledger/aries-framework-go/pkg/internal/mock/kms/legacykms"
	mockprovider "github.com/hyperledger/aries-framework-go/pkg/internal/mock/provider"
	mockstore "github.com/hyperledger/aries-framework-go/pkg/internal/mock/storage"
	mockvdri "github.com/hyperledger/aries-framework-go/pkg/internal/mock/vdri"
	"github.com/hyperledger/aries-framework-go/pkg/store/connection"
)

const (
	MYDID    = "myDID"
	THEIRDID = "theirDID"
)

type updateResult struct {
	action string
	result string
}

func TestServiceNew(t *testing.T) {
	t.Run("test new service - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NoError(t, err)
		require.Equal(t, Coordination, svc.Name())
	})

	t.Run("test new service name - failure", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{
				ErrOpenStoreHandle: fmt.Errorf("error opening the store")}})
		require.Error(t, err)
		require.Contains(t, err.Error(), "open route coordination store")
		require.Nil(t, svc)
	})
}

func TestServiceAccept(t *testing.T) {
	s := &Service{}

	require.Equal(t, true, s.Accept(RequestMsgType))
	require.Equal(t, true, s.Accept(GrantMsgType))
	require.Equal(t, true, s.Accept(KeylistUpdateMsgType))
	require.Equal(t, true, s.Accept(KeylistUpdateResponseMsgType))
	require.Equal(t, true, s.Accept(ForwardMsgType))
	require.Equal(t, false, s.Accept("unsupported msg type"))
}

func TestServiceHandleInbound(t *testing.T) {
	t.Run("test handle outbound ", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(&service.DIDCommMsg{Header: &service.Header{
			ID: msgID,
		}}, "", "")
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})
}

func TestServiceHandleOutbound(t *testing.T) {
	t.Run("test handle outbound ", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
		})
		require.NoError(t, err)

		err = svc.HandleOutbound(nil, "", "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not implemented")
	})
}

func TestServiceRequestMsg(t *testing.T) {
	t.Run("test service handle inbound request msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateRequestMsgPayload(t, msgID), "", "")
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle request msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleRequest(msg, MYDID, THEIRDID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route request message unmarshal")
	})

	t.Run("test service handle request msg - verify outbound message", func(t *testing.T) {
		endpoint := "ws://agent.example.com"
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			InboundEndpointValue:          endpoint,
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateSend: func(msg interface{}, senderVerKey string, des *service.Destination) error {
					res, err := json.Marshal(msg)
					require.NoError(t, err)

					grant := &Grant{}
					err = json.Unmarshal(res, grant)
					require.NoError(t, err)

					require.Equal(t, endpoint, grant.Endpoint)
					require.Equal(t, 1, len(grant.RoutingKeys))

					return nil
				},
			},
		})
		require.NoError(t, err)

		msgID := randomID()

		err = svc.handleRequest(generateRequestMsgPayload(t, msgID), MYDID, THEIRDID)
		require.NoError(t, err)
	})
}

func TestServiceGrantMsg(t *testing.T) {
	t.Run("test service handle inbound grant msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateGrantMsgPayload(t, msgID), "", "")
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle grant msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleGrant(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route grant message unmarshal")
	})
}

func TestServiceUpdateKeyListMsg(t *testing.T) {
	t.Run("test service handle inbound key list update msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateKeyUpdateListMsgPayload(t, msgID, []Update{{
			RecipientKey: "ABC",
			Action:       "add",
		}}), "", "")
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle key list update msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleKeylistUpdate(msg, MYDID, THEIRDID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route key list update message unmarshal")
	})

	t.Run("test service handle request msg - verify outbound message", func(t *testing.T) {
		update := make(map[string]updateResult)
		update["ABC"] = updateResult{action: add, result: success}
		update["XYZ"] = updateResult{action: remove, result: serverError}
		update[""] = updateResult{action: add, result: success}

		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateSend: func(msg interface{}, senderVerKey string, des *service.Destination) error {
					res, err := json.Marshal(msg)
					require.NoError(t, err)

					updateRes := &KeylistUpdateResponse{}
					err = json.Unmarshal(res, updateRes)
					require.NoError(t, err)

					require.Equal(t, len(update), len(updateRes.Updated))

					for _, v := range updateRes.Updated {
						require.Equal(t, update[v.RecipientKey].action, v.Action)
						require.Equal(t, update[v.RecipientKey].result, v.Result)
					}

					return nil
				},
			},
		})
		require.NoError(t, err)

		msgID := randomID()

		var updates []Update
		for k, v := range update {
			updates = append(updates, Update{
				RecipientKey: k,
				Action:       v.action,
			})
		}

		err = svc.handleKeylistUpdate(generateKeyUpdateListMsgPayload(t, msgID, updates), MYDID, THEIRDID)
		require.NoError(t, err)
	})
}

func TestServiceKeylistUpdateResponseMsg(t *testing.T) {
	t.Run("test service handle inbound key list update response msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateKeylistUpdateResponseMsgPayload(t, msgID, []UpdateResponse{{
			RecipientKey: "ABC",
			Action:       "add",
			Result:       success,
		}}), "", "")
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle key list update response msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleKeylistUpdateResponse(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "route keylist update response message unmarshal")
	})
}

func TestServiceForwardMsg(t *testing.T) {
	t.Run("test service handle inbound forward msg - success", func(t *testing.T) {
		to := randomID()
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		err = svc.routeStore.Put(to, []byte("did:example:123"))
		require.NoError(t, err)

		msgID := randomID()

		id, err := svc.HandleInbound(generateForwardMsgPayload(t, msgID, to, nil), "", "")
		require.NoError(t, err)
		require.Equal(t, msgID, id)
	})

	t.Run("test service handle forward msg - success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		msg := &service.DIDCommMsg{Payload: []byte("invalid json")}

		err = svc.handleForward(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "forward message unmarshal")
	})

	t.Run("test service handle forward msg - route key fetch fail", func(t *testing.T) {
		to := randomID()
		msgID := randomID()

		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		err = svc.handleForward(generateForwardMsgPayload(t, msgID, to, nil))
		require.Error(t, err)
		require.Contains(t, err.Error(), "route key fetch")
	})

	t.Run("test service handle forward msg - validate forward message content", func(t *testing.T) {
		to := randomID()
		msgID := randomID()
		invalidDID := "did:error:123"

		content := "packed message destined to the recipient through router"
		msg := generateForwardMsgPayload(t, msgID, to, content)

		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateForward: func(msg interface{}, des *service.Destination) error {
					require.Equal(t, content, msg)

					return nil
				},
			},
			VDRIRegistryValue: &mockvdri.MockVDRIRegistry{
				ResolveFunc: func(didID string, opts ...vdri.ResolveOpts) (doc *did.Doc, e error) {
					if didID == invalidDID {
						return nil, errors.New("invalid")
					}
					return mockdiddoc.GetMockDIDDoc(), nil
				},
			},
		})
		require.NoError(t, err)

		err = svc.routeStore.Put(dataKey(to), []byte("did:example:123"))
		require.NoError(t, err)

		err = svc.handleForward(msg)
		require.NoError(t, err)

		err = svc.routeStore.Put(dataKey(to), []byte(invalidDID))
		require.NoError(t, err)

		err = svc.handleForward(msg)
		require.Error(t, err)
		require.Contains(t, err.Error(), "get destination")
	})
}

func TestSendRequest(t *testing.T) {
	t.Run("test success", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateSendToDID: func(msg interface{}, myDID, theirDID string) error {
					require.Equal(t, myDID, MYDID)
					require.Equal(t, theirDID, THEIRDID)
					return nil
				}}})
		require.NoError(t, err)

		reqID, err := svc.SendRequest(MYDID, THEIRDID)
		require.NoError(t, err)
		require.NotEmpty(t, reqID)
	})

	t.Run("test error from send to did", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateSendToDID: func(msg interface{}, myDID, theirDID string) error {
					return fmt.Errorf("error send")
				}}})
		require.NoError(t, err)

		_, err = svc.SendRequest(MYDID, THEIRDID)
		require.Error(t, err)
		require.Contains(t, err.Error(), "error send")
	})
}

func TestRegister(t *testing.T) {
	t.Run("test register route - success", func(t *testing.T) {
		msgID := make(chan string)

		s := make(map[string][]byte)
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          &mockstore.MockStoreProvider{Store: &mockstore.MockStore{Store: s}},
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateSendToDID: func(msg interface{}, myDID, theirDID string) error {
					require.Equal(t, myDID, MYDID)
					require.Equal(t, theirDID, THEIRDID)

					request, ok := msg.(*Request)
					require.True(t, ok)

					msgID <- request.ID
					return nil
				}}})
		require.NoError(t, err)

		connRec := &connection.Record{
			ConnectionID: "conn1", MyDID: MYDID, TheirDID: THEIRDID, State: "complete"}
		connBytes, err := json.Marshal(connRec)
		require.NoError(t, err)
		s["conn_conn1"] = connBytes

		go func() {
			id := <-msgID
			require.NoError(t, svc.handleGrant(generateGrantMsgPayload(t, id)))
		}()

		err = svc.Register("conn1")
		require.NoError(t, err)

		err = svc.Register("")
		require.Error(t, err)
		require.Contains(t, err.Error(), "router is already registered")
	})

	t.Run("test register route - timeout error", func(t *testing.T) {
		s := make(map[string][]byte)
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          &mockstore.MockStoreProvider{Store: &mockstore.MockStore{Store: s}},
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue:       &mockdispatcher.MockOutbound{}})
		require.NoError(t, err)

		connRec := &connection.Record{
			ConnectionID: "conn2", MyDID: MYDID, TheirDID: THEIRDID, State: "complete"}
		connBytes, err := json.Marshal(connRec)
		require.NoError(t, err)
		s["conn_conn2"] = connBytes

		err = svc.Register("conn2")
		require.Error(t, err)
		require.Contains(t, err.Error(), "timeout waiting for grant from the router")
	})

	t.Run("test register route - router connection not found", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue:          mockstore.NewMockStoreProvider(),
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateSendToDID: func(msg interface{}, myDID, theirDID string) error {
					return fmt.Errorf("error send")
				}}})
		require.NoError(t, err)

		err = svc.Register("conn1")
		require.Error(t, err)
		require.Contains(t, err.Error(), ErrConnectionNotFound.Error())
	})

	t.Run("test register route - router connection fetch error", func(t *testing.T) {
		svc, err := New(&mockprovider.Provider{
			StorageProviderValue: &mockstore.MockStoreProvider{
				Store: &mockstore.MockStore{ErrGet: fmt.Errorf("get error")}},
			TransientStorageProviderValue: mockstore.NewMockStoreProvider(),
			KMSValue:                      &mockkms.CloseableKMS{},
			OutboundDispatcherValue: &mockdispatcher.MockOutbound{
				ValidateSendToDID: func(msg interface{}, myDID, theirDID string) error {
					return fmt.Errorf("error send")
				}}})
		require.NoError(t, err)

		err = svc.Register("conn1")
		require.Error(t, err)
		require.Contains(t, err.Error(), "fetch router connection id")
	})
}

func generateRequestMsgPayload(t *testing.T, id string) *service.DIDCommMsg {
	requestBytes, err := json.Marshal(&Request{
		Type: RequestMsgType,
		ID:   id,
	})
	require.NoError(t, err)

	didMsg, err := service.NewDIDCommMsg(requestBytes)
	require.NoError(t, err)

	return didMsg
}

func generateGrantMsgPayload(t *testing.T, id string) *service.DIDCommMsg {
	grantBytes, err := json.Marshal(&Grant{
		Type: GrantMsgType,
		ID:   id,
	})
	require.NoError(t, err)

	didMsg, err := service.NewDIDCommMsg(grantBytes)
	require.NoError(t, err)

	return didMsg
}

func generateKeyUpdateListMsgPayload(t *testing.T, id string, updates []Update) *service.DIDCommMsg {
	requestBytes, err := json.Marshal(&KeylistUpdate{
		Type:    KeylistUpdateMsgType,
		ID:      id,
		Updates: updates,
	})
	require.NoError(t, err)

	didMsg, err := service.NewDIDCommMsg(requestBytes)
	require.NoError(t, err)

	return didMsg
}

func generateKeylistUpdateResponseMsgPayload(t *testing.T, id string, updates []UpdateResponse) *service.DIDCommMsg {
	respBytes, err := json.Marshal(&KeylistUpdateResponse{
		Type:    KeylistUpdateResponseMsgType,
		ID:      id,
		Updated: updates,
	})
	require.NoError(t, err)

	didMsg, err := service.NewDIDCommMsg(respBytes)
	require.NoError(t, err)

	return didMsg
}

func generateForwardMsgPayload(t *testing.T, id, to string, msg interface{}) *service.DIDCommMsg {
	requestBytes, err := json.Marshal(&Forward{
		Type: ForwardMsgType,
		ID:   id,
		To:   to,
		Msg:  msg,
	})
	require.NoError(t, err)

	didMsg, err := service.NewDIDCommMsg(requestBytes)
	require.NoError(t, err)

	return didMsg
}

func randomID() string {
	return uuid.New().String()
}
