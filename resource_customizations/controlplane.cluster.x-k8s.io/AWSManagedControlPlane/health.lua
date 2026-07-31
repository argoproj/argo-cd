local health_status = {
    status = "Progressing",
    message = "Provisioning..."
}

-- If .status is nil, then the control plane is not ready
if obj.status == nil then
    return health_status
end

-- A problem reported through .status.failureMessage is terminal and will not clear by waiting,
-- so surface it rather than reporting Progressing indefinitely. This covers a cluster in the
-- FAILED state as well as one that EKS auto-upgraded out of standard support, which stays
-- broken until .spec.version is corrected by a human. It is checked before .status.conditions
-- so that a terminal failure is still reported when conditions are absent.
-- https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/v2.11.0/pkg/cloud/services/eks/cluster.go#L207-L235
if obj.status.failureMessage ~= nil and obj.status.failureMessage ~= "" then
    health_status.status = "Degraded"
    health_status.message = obj.status.failureMessage
    return health_status
end

-- Without conditions there is nothing left to inspect, so the control plane is not ready
if obj.status.conditions == nil then
    return health_status
end

-- An in-place update of the EKS control plane keeps both .status.ready and the Ready condition
-- True, because the API server stays reachable while it happens. The provider signals the
-- ongoing update through a dedicated condition instead, so this has to be checked before Ready
-- to avoid reporting Healthy on a control plane that has not converged yet.
-- https://github.com/kubernetes-sigs/cluster-api-provider-aws/blob/v2.11.0/pkg/cloud/services/eks/cluster.go#L239-L240
for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "EKSControlPlaneUpdating" and condition.status == "True" then
        health_status.status = "Progressing"
        health_status.message = "EKS control plane is updating"
        return health_status
    end
end

-- Accumulator for the error messages (could be multiple conditions in error state)
err_msg = ""

-- Iterate over the conditions to determine the health status
for i, condition in ipairs(obj.status.conditions) do
    -- Check if the Ready condition is True, then the control plane is ready
    if condition.type == "Ready" and condition.status == "True" then
        health_status.status = "Healthy"
        health_status.message = "Control plane is ready"
        return health_status
    end

    -- If we have a condition that is False and has an Error severity, then the control plane is in a degraded state
    if condition.status == "False" and condition.severity == "Error" then
        health_status.status = "Degraded"
        err_msg = err_msg .. condition.message .. " "
    end
end

-- If we have any error conditions, then the control plane is in a degraded state
if health_status.status == "Degraded" then
    health_status.message = err_msg
    return health_status
end

-- If .status.ready is False, then the control plane is not ready
if obj.status.ready == false then
    health_status.status = "Progressing"
    health_status.message = "Control plane is not ready (.status.ready is false)"
    return health_status
end

-- If we reach this point, then the control plane is not ready and we don't have any error conditions
health_status.status = "Progressing"
health_status.message = "Control plane is not ready"

return health_status
