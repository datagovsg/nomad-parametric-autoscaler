package resources

import (
	"fmt"

	"github.com/datagovsg/nomad-parametric-autoscaler/logging"
	nomad "github.com/hashicorp/nomad/api"
)

type NomadClientPlan struct {
	Address   string `json:"Address"`
	JobName   string `json:"JobName"`
	NomadPath string `json:"NomadPath"`
	MaxCount  int    `json:"MaxCount"`
	MinCount  int    `json:"MinCount"`
}

// NomadClient contains details to access our nomad client
type NomadClient struct {
	JobName   string
	NomadPath string
	client    nomadClient
	MaxCount  int
	MinCount  int
	address   string
}

// wrapper? maybe we dont need this
type nomadClient struct {
	nomad *nomad.Client
}

func (ncp NomadClientPlan) ApplyPlan(vc VaultClient) (*NomadClient, error) {
	return NewNomadClient(vc, ncp.Address, ncp.JobName, ncp.MinCount, ncp.MaxCount, ncp.NomadPath)
}

// NewNomadClient is a factory that produces a new NewNomadClient
func NewNomadClient(vc VaultClient, addr string, name string, minCount int, maxCount int, nomadPath string) (*NomadClient, error) {
	nc := nomad.DefaultConfig()
	nc.Address = addr

	token, err := vc.GetNomadToken(nomadPath)
	if err != nil {
		logging.Error(err.Error())
		return nil, err
	}
	nc.SecretID = token

	client, err := nomad.NewClient(nc)
	if err != nil {
		logging.Error(err.Error())
		return nil, err
	}

	return &NomadClient{
		JobName:   name,
		NomadPath: nomadPath,
		MaxCount:  maxCount,
		MinCount:  minCount,
		client: nomadClient{
			nomad: client,
		},
		address: addr,
	}, nil
}

// GetTaskGroupCount retrieves the jobspec to check task group count
func (nc NomadClient) GetTaskGroupCount() (int, error) {
	job, err := nc.getNomadJob()
	if err != nil {
		return 0, err
	}
	return *job.TaskGroups[0].Count, nil
}

// Scale get json -> find number -> change number, add vault token -> convert to json
func (nc NomadClient) Scale(newCount int, vc *VaultClient) error {
	newCount = nc.getValidScaleCount(newCount)
	job, err := nc.getNomadJob()
	if err != nil {
		return err
	}

	tg := job.TaskGroups[0]
	oldCount := *tg.Count
	*tg.Count = newCount
	*job.VaultToken = vc.GetVaultToken()

	_, _, err = nc.client.nomad.Jobs().Register(job, &nomad.WriteOptions{})
	if err != nil {
		logging.Error(err.Error())
		return err
	}

	count, _ := nc.GetTaskGroupCount()
	logging.Info("[scaling log] Nomad job: %s. Old: %d. New: %d", nc.JobName, oldCount, count)

	return nil
}

// RestartNomadAlloc - restart any Nomad allocation of the associated job in NomadClient
func (nc NomadClient) RestartNomadAlloc() error {
	allocs, _, err := nc.client.nomad.Jobs().Allocations(nc.JobName, true, nil)
	if err != nil {
		return err
	}
	if len(allocs) <= 0 {
		return fmt.Errorf("No allocations to restart")
	}
	allocID := ""
	for i := range allocs {
		if allocs[i].ClientStatus == "running" {
			allocID = allocs[i].ID
			break
		}
	}
	if allocID == "" {
		return fmt.Errorf("No running allocations to restart")
	}
	alloc, _, err := nc.client.nomad.Allocations().Info(allocID, nil)
	if err != nil {
		return err
	}
	logging.Info("[restart log] stopping %s", alloc.ID)
	// The Nomad server will restore the allocation shortly after if it's stop,
	// making Stop effectively a restart.
	_, err = nc.client.nomad.Allocations().Stop(alloc, nil)
	if err != nil {
		return err
	}
	return nil
}

// getNomadJob - private method that fetches the nomad jobspec for this resource
func (nc NomadClient) getNomadJob() (*nomad.Job, error) {
	job, _, err := nc.client.nomad.Jobs().Info(nc.JobName, &nomad.QueryOptions{})
	if err != nil {
		return nil, err
	}
	return job, nil
}

func (nc NomadClient) getValidScaleCount(newCount int) int {
	if newCount > nc.MaxCount {
		newCount = nc.MaxCount
	} else if newCount < nc.MinCount {
		newCount = nc.MinCount
	}
	return newCount
}

func (nc NomadClient) RecreatePlan() NomadClientPlan {
	return NomadClientPlan{
		Address:   nc.address,
		JobName:   nc.JobName,
		NomadPath: nc.NomadPath,
		MaxCount:  nc.MaxCount,
		MinCount:  nc.MinCount,
	}
}
