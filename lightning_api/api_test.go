package lightning_api

import (
	"testing"

	entities "github.com/bolt-observer/go_common/entities"
	utils "github.com/bolt-observer/go_common/utils"
)

func TestApiSelection(t *testing.T) {

	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	// ApiType in NewApi() is a preference that can be overriden through data

	// Invalid API type
	api := NewApi(ApiType(3), func() (*entities.Data, error) {
		return &data, nil
	})

	if api != nil {
		t.Fatalf("API should be nil")
	}

	// Use gRPC
	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok := api.(*LndGrpcLightningApi)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	// Use REST
	api = NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	v := int(LND_GRPC)
	data.ApiType = &v

	api = NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningApi)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

	v = int(LND_REST)
	data.ApiType = &v

	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndRestLightningApi)
	if !ok {
		t.Fatalf("Should be LND_REST")
	}

	// Invalid type
	v = 3
	data.ApiType = &v

	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	_, ok = api.(*LndGrpcLightningApi)
	if !ok {
		t.Fatalf("Should be LND_GRPC")
	}

}

func TestGrpcDoesNotOpenConnection(t *testing.T) {
	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	api := NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api = NewApi(LND_GRPC, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}
}

func TestRestDoesNotOpenConnection(t *testing.T) {
	cert := utils.ObtainCert("bolt.observer:443")
	dummyMac := "0201036c6e640224030a10f1c3ac8f073a46b6474e24b780a96c3f1201301a0c0a04696e666f12047265616400022974696d652d6265666f726520323032322d30382d30385430383a31303a30342e38383933303336335a00020e69706164647220312e322e332e34000006201495fe7fe048b47ff26abd66a56393869aec2dcb249594ebea44d398f58f26ec"

	data := entities.Data{
		PubKey:            "030f7b46defcec976ed516de5e7841bdcb7a19bb388b679ec9dba4bb526e93efb0",
		MacaroonHex:       dummyMac,
		CertificateBase64: cert,
		Endpoint:          "bolt.observer:443",
	}

	api := NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}

	data.Endpoint = "burek:444"
	api = NewApi(LND_REST, func() (*entities.Data, error) {
		return &data, nil
	})

	if api == nil {
		t.Fatalf("API should not be nil")
	}
}
