package model

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"path/filepath"
)

// RoleManifest represents a collection of roles
type RoleManifest struct {
	Roles []*Role `yaml:"roles"`

	manifestFilePath string
}

// Role represents a collection of jobs that are colocated on a container
type Role struct {
	Name        string     `yaml:"name"`
	Jobs        []*Job     `yaml:"_,omitempty"`
	Scripts     []string   `yaml:"scripts"`
	IsTask      bool       `yaml:"is_task,omitempty"`
	JobNameList []*roleJob `yaml:"jobs"`

	rolesManifest *RoleManifest
}

type roleJob struct {
	Name string `yaml:"name"`
}

// LoadRoleManifest loads a yaml manifest that details how jobs get grouped into roles
func LoadRoleManifest(manifestFilePath string, release *Release) (*RoleManifest, error) {
	manifestContents, err := ioutil.ReadFile(manifestFilePath)
	if err != nil {
		return nil, err
	}

	rolesManifest := RoleManifest{}
	rolesManifest.manifestFilePath = manifestFilePath
	if err := yaml.Unmarshal(manifestContents, &rolesManifest); err != nil {
		return nil, err
	}

	for _, role := range rolesManifest.Roles {
		role.rolesManifest = &rolesManifest
		role.Jobs = make([]*Job, len(role.JobNameList))

		for idx, roleJob := range role.JobNameList {
			job, err := release.LookupJob(roleJob.Name)
			if err != nil {
				return nil, err
			}

			role.Jobs[idx] = job
		}
	}

	return &rolesManifest, nil
}

func (r *Role) GetScriptPaths() map[string]string {
	result := map[string]string{}

	if r.Scripts == nil {
		return result
	}

	for _, script := range r.Scripts {
		result[script] = filepath.Join(filepath.Dir(r.rolesManifest.manifestFilePath), script)
	}

	return result

}