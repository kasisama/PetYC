package admin

import (
	"github.com/gin-gonic/gin"
	"qq-pet-saas/security"
)

const currentTourVersion = 1

func RegisterOnboardingRoutes(group *gin.RouterGroup) {
	group.GET("/onboarding/status", getOnboardingStatus)
	group.POST("/onboarding/setup-complete", completeOnboardingSetup)
	group.PUT("/onboarding/tour", completeOnboardingTour)
}

func getOnboardingStatus(c *gin.Context) {
	state, err := security.LoadOnboardingState()
	if err != nil {
		Error(c, codeInternalError, "新手引导状态读取失败")
		return
	}
	Success(c, gin.H{"setup_completed": state.SetupCompleted, "tour_version_completed": state.TourVersionCompleted, "current_tour_version": currentTourVersion})
}
func completeOnboardingSetup(c *gin.Context) {
	if err := security.CompleteSetup(); err != nil {
		Error(c, codeInternalError, "首启状态保存失败")
		return
	}
	Success(c, gin.H{"message": "首次配置已完成"})
}
func completeOnboardingTour(c *gin.Context) {
	var request struct {
		Version int `json:"version"`
	}
	if err := c.ShouldBindJSON(&request); err != nil || request.Version < 1 || request.Version > currentTourVersion {
		Error(c, codeInvalidPayload, "导览版本无效")
		return
	}
	if err := security.CompleteTour(request.Version); err != nil {
		Error(c, codeInternalError, "导览状态保存失败")
		return
	}
	Success(c, gin.H{"message": "新手导览状态已保存"})
}
