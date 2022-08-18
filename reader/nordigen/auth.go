package nordigen

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
	"github.com/martinohansen/ynabber"
)

type Authorization struct {
	Client    nordigen.Client
	BankID    string
	EndUserId string
}

func (auth Authorization) Store() string {
	return fmt.Sprintf("%s/%s.json", ynabber.DataDir(), auth.EndUserId)
}

// AuthorizationWrapper tries to get requisition from disk, if it fails it will
// create a new and store that one to disk.
func (auth Authorization) Wrapper() (nordigen.Requisition, error) {
	requisitionFile, err := os.ReadFile(auth.Store())
	if errors.Is(err, os.ErrNotExist) {
		log.Print("Requisition is not found")
		return auth.CreateAndSave()
	} else if err != nil {
		return nordigen.Requisition{}, fmt.Errorf("ReadFile: %w", err)
	}

	var requisition nordigen.Requisition
	err = json.Unmarshal(requisitionFile, &requisition)
	if err != nil {
		log.Print("Failed to parse requisition file")
		return auth.CreateAndSave()
	}

	switch requisition.Status {
	case "EX":
		log.Printf("Requisition is expired")
		return auth.CreateAndSave()
	case "LN":
		return requisition, nil
	default:
		log.Printf("Unsupported requisition status: %s", requisition.Status)
		return auth.CreateAndSave()
	}
}

func (auth Authorization) CreateAndSave() (nordigen.Requisition, error) {
	log.Print("Creating new requisition...")
	requisition, err := auth.Create()
	if err != nil {
		return nordigen.Requisition{}, fmt.Errorf("AuthorizationCreate: %w", err)
	}
	err = auth.Save(requisition)
	if err != nil {
		log.Printf("Failed to write requisition to disk: %s", err)
	}
	log.Printf("Requisition stored for reuse: %s", auth.Store())
	return requisition, nil
}

func (auth Authorization) Save(requisition nordigen.Requisition) error {
	requisitionFile, err := json.Marshal(requisition)
	if err != nil {
		return err
	}

	err = os.WriteFile(auth.Store(), requisitionFile, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (auth Authorization) Create() (nordigen.Requisition, error) {
	requisition := nordigen.Requisition{
		Redirect:      "https://raw.githubusercontent.com/martinohansen/ynabber/main/ok.html",
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
