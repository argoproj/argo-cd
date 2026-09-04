-- ControlPlaneComponent represents a single component of a hosted control plane (e.g. kube-apiserver).
-- It is reconciled by CPOv2, the second generation of the HyperShift Control Plane Operator.
-- Each component reports its own availability and rollout state independently.
--
-- Documentation:
--   CPOv2 architecture: https://github.com/openshift/hypershift/blob/main/support/controlplane-component/README.md
--
-- Condition types and constants are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/controlplanecomponent_types.go
--
-- Condition types:
--   Available     (True/False) - Whether the component is currently available
--   RolloutComplete (True/False) - Whether the component's current rollout has finished
--
-- This resource exposes status.observedGeneration, which tracks which generation of the
-- parent HostedControlPlane spec has been reconciled by this component.
-- See: https://alenkacz.medium.com/kubernetes-operator-best-practices-implementing-observedgeneration-250728868792
--
-- ArgoCD health mapping:
--   observedGeneration != metadata.generation   => Progressing (spec not yet reconciled)
--   Available=False                             => Degraded
--   RolloutComplete=False                       => Progressing
--   Available=True AND RolloutComplete=True     => Healthy
--   No conditions yet                           => Progressing
local hs = {}

if obj.status == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for ControlPlaneComponent status"
  return hs
end

-- Generation check: controller has not yet observed the latest spec
if obj.metadata.generation ~= nil and obj.status.observedGeneration ~= nil then
  if obj.status.observedGeneration ~= obj.metadata.generation then
    hs.status = "Progressing"
    hs.message = "Waiting for spec update to be observed by controller"
    return hs
  end
end

if obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for ControlPlaneComponent conditions"
  return hs
end

-- Unavailable component is degraded
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Available" and condition.status == "False" then
    hs.status = "Degraded"
    hs.message = condition.message
    return hs
  end
end

-- Rollout still in flight
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "RolloutComplete" and condition.status == "False" then
    hs.status = "Progressing"
    hs.message = condition.message
    return hs
  end
end

-- Both Available and RolloutComplete must be True
local available = false
local rolloutComplete = false
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Available" and condition.status == "True" then
    available = true
  end
  if condition.type == "RolloutComplete" and condition.status == "True" then
    rolloutComplete = true
  end
end

if available and rolloutComplete then
  hs.status = "Healthy"
  hs.message = "Component is available and rollout is complete"
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for ControlPlaneComponent to become available"
return hs
