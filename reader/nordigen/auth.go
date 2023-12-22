package nordigen

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/martinohansen/ynabber"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
)

type Authorization struct {
	BankID string
	Client nordigen.Client
	File   string
}

// Store returns a clean path to the requisition file
func (auth Authorization) Store() string {
	return path.Clean(auth.File)
}

// AuthorizationWrapper tries to get requisition from disk, if it fails it will
// create a new and store that one to disk.
func (auth Authorization) Wrapper(cfg ynabber.Config) (nordigen.Requisition, error) {
	requisitionFile, err := os.ReadFile(auth.Store())
	if errors.Is(err, os.ErrNotExist) {
		log.Print("Requisition is not found")
		return auth.CreateAndSave(cfg)
	} else if err != nil {
		return nordigen.Requisition{}, fmt.Errorf("ReadFile: %w", err)
	}

	var requisition nordigen.Requisition
	err = json.Unmarshal(requisitionFile, &requisition)
	if err != nil {
		log.Print("Failed to parse requisition file")
		return auth.CreateAndSave(cfg)
	}

	switch requisition.Status {
	case "EX":
		log.Printf("Requisition is expired")
		return auth.CreateAndSave(cfg)
	case "LN":
		return requisition, nil
	default:
		log.Printf("Unsupported requisition status: %s", requisition.Status)
		return auth.CreateAndSave(cfg)
	}
}

func (auth Authorization) CreateAndSave(cfg ynabber.Config) (nordigen.Requisition, error) {
	log.Print("Creating new requisition...")
	requisition, err := auth.Create(cfg)
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

func (auth Authorization) Create(cfg ynabber.Config) (nordigen.Requisition, error) {
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

	auth.Notify(cfg, r)
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

func (auth Authorization) Notify(cfg ynabber.Config, r nordigen.Requisition) {
	if cfg.NotificationScript != "" {
		cmd := exec.Command(cfg.NotificationScript, r.Link)
		_, err := cmd.Output()
		if err != nil {
			log.Println("Could not notify user about new requisition: ", err)
		}
	} else {
		log.Println("No Notification Script set")
	}
}
