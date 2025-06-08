package nordigen

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"github.com/frieser/nordigen-go-lib/v2"
)

const RequisitionRedirect = "https://raw.githubusercontent.com/martinohansen/ynabber/main/ok.html"

// requisitionStore returns a clean path to the requisition file
func (r Reader) requisitionStore() string {
	// Use BankID or RequisitionFile as filename
	var file string
	if r.Config.RequisitionFile == "" {
		file = r.Config.BankID
	} else {
		file = r.Config.RequisitionFile
	}

	return path.Clean(fmt.Sprintf("%s/%s.json", r.DataDir, file))
}

// Requisition tries to get requisition from disk, if it fails it will create a
// new and store that one to disk.
func (r Reader) Requisition() (nordigen.Requisition, error) {
	requisitionFile, err := os.ReadFile(r.requisitionStore())
	if errors.Is(err, os.ErrNotExist) {
		r.logger.Info("requisition is not found")
		return r.createRequisition()
	} else if err != nil {
		return nordigen.Requisition{}, fmt.Errorf("ReadFile: %w", err)
	}

	var requisition nordigen.Requisition
	err = json.Unmarshal(requisitionFile, &requisition)
	if err != nil {
		r.logger.Error("parsing requisition file", "error", err)
		return r.createRequisition()
	}

	switch requisition.Status {
	case "EX":
		// Create a new requisition if expired
		r.logger.Info("requisition is expired")
		return r.createRequisition()
	case "LN":
		// Return requisition if it's still valid
		return requisition, nil
	default:
		// Handle unknown status by recreating requisition
		r.logger.Info("unsupported requisition status", "status", requisition.Status)
		return r.createRequisition()
	}
}

func (r Reader) saveRequisition(requisition nordigen.Requisition) error {
	requisitionFile, err := json.Marshal(requisition)
	if err != nil {
		return err
	}

	err = os.WriteFile(r.requisitionStore(), requisitionFile, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (r Reader) createRequisition() (nordigen.Requisition, error) {
	requisition, err := r.Client.CreateRequisition(nordigen.Requisition{
		Redirect:      RequisitionRedirect,
		Reference:     strconv.Itoa(int(time.Now().Unix())),
		Agreement:     "",
		InstitutionId: r.Config.BankID,
	})
	if err != nil {
		return nordigen.Requisition{}, fmt.Errorf("CreateRequisition: %w", err)
	}

	if err := r.requisitionHook(requisition); err != nil {
		return nordigen.Requisition{}, fmt.Errorf("running requisition hook: %w", err)
	}
	r.logger.Info("initiate requisition by going to", "link", requisition.Link)

	// Keep waiting for the user to accept the requisition
	for requisition.Status != "LN" {
		requisition, err = r.Client.GetRequisition(requisition.Id)
		if err != nil {
			return nordigen.Requisition{}, fmt.Errorf("GetRequisition: %w", err)
		}
		time.Sleep(2 * time.Second)
	}

	// Store requisition on disk
	err = r.saveRequisition(requisition)
	if err != nil {
		r.logger.Error("writing requisition to disk", "error", err)
	}

	return requisition, nil
}

// requisitionHook executes the hook and returns its exit code
func (r Reader) requisitionHook(req nordigen.Requisition) error {
	if r.Config.RequisitionHook != "" {
		cmd := exec.Command(r.Config.RequisitionHook, req.Status, req.Link)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("executing hook: %w, output: %s", err, output)
		}
		r.logger.Info("requisition hook output", "output", string(output))
		return nil
	}
	return nil
}
