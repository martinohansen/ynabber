package nordigen

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

const redirectPort = ":3000"

type Authorization struct {
	Client    nordigen.Client
	BankID    string
	EndUserId string
	Store     string
}

func (auth Authorization) DiskStore() string {
	return fmt.Sprintf("%s/%s.json", ynabber.DataDir(), auth.EndUserId)
}

// AuthorizationWrapper gets and stores a working requisition
func (auth Authorization) Wrapper() (nordigen.Requisition, error) {
	requisition, err := auth.Client.GetRequisition(auth.Read().Id)
	if err != nil {
		log.Printf("Cant get requisition: %s", err)
		return auth.CreateAndSave()
	}
	if requisition.Status == "LN" {
		log.Print("Reusing existing requisition")
		return requisition, nil
	} else {
		log.Print("Unable to handle requisition")
		return auth.CreateAndSave()
	}
}

func (auth Authorization) Read() nordigen.Requisition {
	var requisition nordigen.Requisition
	switch auth.Store {
	case "disk":
		log.Print("Trying to read requisition from disk")
		store, err := os.ReadFile(auth.DiskStore())
		if err != nil {
			log.Print("No requisition on disk is found")
			return requisition
		}
		err = json.Unmarshal(store, &requisition)
		if err != nil {
			log.Print("Unsupported requisition format")
			return requisition
		}
	}
	return requisition
}

func (auth Authorization) CreateAndSave() (nordigen.Requisition, error) {
	log.Print("Creating new requisition...")
	requisition, err := auth.Create()
	if err != nil {
		return nordigen.Requisition{}, fmt.Errorf("AuthorizationCreate: %w", err)
	}
	err = auth.Save(requisition)
	if err != nil {
		log.Printf("Failed to save requisition: %s", err)
	}
	return requisition, nil
}

func (auth Authorization) Save(requisition nordigen.Requisition) error {
	requisition_blob, err := json.Marshal(requisition)
	if err != nil {
		return err
	}

	switch auth.Store {
	case "disk":
		err = os.WriteFile(auth.DiskStore(), requisition_blob, 0644)
		if err != nil {
			return fmt.Errorf("cant store requisition on disk")
		}
	default:
		return fmt.Errorf("unsupported disk store: %s", auth.Store)
	}

	return nil
}

func (auth Authorization) Create() (nordigen.Requisition, error) {
	requisition := nordigen.Requisition{
		Redirect:      "http://localhost" + redirectPort,
		Reference:     strconv.Itoa(int(time.Now().Unix())),
		Agreement:     "",
		InstitutionId: auth.BankID,
	}

	r, err := auth.Client.CreateRequisition(requisition)
	if err != nil {
		return nordigen.Requisition{}, fmt.Errorf("CreateRequisition: %w", err)
	}

	log.Printf("Initiate requisition by going to: %s", r.Link)

	// Keep waiting for the user to accept the requisition
	for r.Status != "LN" {
		r, err = auth.Client.GetRequisition(r.Id)

		if err != nil {
			return nordigen.Requisition{}, fmt.Errorf("GetRequisition: %w", err)
		}
		time.Sleep(1 * time.Second)
	}

	return r, nil
}
