package app

import (
	"omniflow/internal/common"
	"time"

	"go.temporal.io/sdk/workflow"
)

func ShippingChildWorkflow(ctx workflow.Context, shipment common.Shipment) (string, error) {
	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{StartToCloseTimeout: time.Minute})
	var shipActs *ShippingActivities
	var trackingID string
	err := workflow.ExecuteActivity(ctx, shipActs.GenerateShippingLabel, shipment).Get(ctx, &trackingID)
	return trackingID, err
}
