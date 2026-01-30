package agentpod

import (
	"errors"

	"github.com/anthropics/agentsmesh/backend/internal/domain/agentpod"
	"gorm.io/gorm"
)

var (
	ErrAutopilotControllerNotFound = errors.New("autopilot pod not found")
)

// AutopilotControllerService handles AutopilotController operations
type AutopilotControllerService struct {
	db *gorm.DB
}

// NewAutopilotControllerService creates a new AutopilotController service
func NewAutopilotControllerService(db *gorm.DB) *AutopilotControllerService {
	return &AutopilotControllerService{db: db}
}

// GetAutopilotController retrieves a AutopilotController by organization ID and key
func (s *AutopilotControllerService) GetAutopilotController(orgID int64, autopilotPodKey string) (*agentpod.AutopilotController, error) {
	var pod agentpod.AutopilotController
	err := s.db.Where("organization_id = ? AND autopilot_controller_key = ?", orgID, autopilotPodKey).First(&pod).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAutopilotControllerNotFound
		}
		return nil, err
	}
	return &pod, nil
}

// ListAutopilotControllers lists all AutopilotControllers for an organization
func (s *AutopilotControllerService) ListAutopilotControllers(orgID int64) ([]*agentpod.AutopilotController, error) {
	var pods []*agentpod.AutopilotController
	err := s.db.Where("organization_id = ?", orgID).Order("created_at DESC").Find(&pods).Error
	if err != nil {
		return nil, err
	}
	return pods, nil
}

// CreateAutopilotController creates a new AutopilotController
func (s *AutopilotControllerService) CreateAutopilotController(pod *agentpod.AutopilotController) error {
	return s.db.Create(pod).Error
}

// UpdateAutopilotController updates an existing AutopilotController
func (s *AutopilotControllerService) UpdateAutopilotController(pod *agentpod.AutopilotController) error {
	return s.db.Save(pod).Error
}

// UpdateAutopilotControllerStatus updates the status fields of a AutopilotController
func (s *AutopilotControllerService) UpdateAutopilotControllerStatus(autopilotPodKey string, updates map[string]interface{}) error {
	return s.db.Model(&agentpod.AutopilotController{}).
		Where("autopilot_controller_key = ?", autopilotPodKey).
		Updates(updates).Error
}

// GetIterations retrieves all iterations for a AutopilotController
func (s *AutopilotControllerService) GetIterations(autopilotPodID int64) ([]*agentpod.AutopilotIteration, error) {
	var iterations []*agentpod.AutopilotIteration
	err := s.db.Where("autopilot_controller_id = ?", autopilotPodID).Order("iteration ASC").Find(&iterations).Error
	if err != nil {
		return nil, err
	}
	return iterations, nil
}

// CreateIteration creates a new iteration record
func (s *AutopilotControllerService) CreateIteration(iteration *agentpod.AutopilotIteration) error {
	return s.db.Create(iteration).Error
}

// GetAutopilotControllerByKey retrieves a AutopilotController by key only (for internal use)
func (s *AutopilotControllerService) GetAutopilotControllerByKey(autopilotPodKey string) (*agentpod.AutopilotController, error) {
	var pod agentpod.AutopilotController
	err := s.db.Where("autopilot_controller_key = ?", autopilotPodKey).First(&pod).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAutopilotControllerNotFound
		}
		return nil, err
	}
	return &pod, nil
}

// GetActiveAutopilotControllerForPod retrieves active AutopilotController for a pod
func (s *AutopilotControllerService) GetActiveAutopilotControllerForPod(podKey string) (*agentpod.AutopilotController, error) {
	var pod agentpod.AutopilotController
	err := s.db.Where("pod_key = ? AND phase NOT IN (?, ?, ?)",
		podKey,
		agentpod.AutopilotPhaseCompleted,
		agentpod.AutopilotPhaseFailed,
		agentpod.AutopilotPhaseStopped,
	).First(&pod).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAutopilotControllerNotFound
		}
		return nil, err
	}
	return &pod, nil
}
