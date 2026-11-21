-- NodePool represents a scalable set of worker nodes attached to a HostedCluster.
-- Each NodePool manages a homogeneous group of machines with a single architecture and platform config.
--
-- Documentation:
--   API reference:  https://hypershift-docs.netlify.app/reference/api/
--   NodePool guide: https://hypershift-docs.netlify.app/how-to/aws/create-aws-hosted-cluster/
--
-- Condition types and constants are defined in:
--   https://github.com/openshift/hypershift/blob/main/api/hypershift/v1beta1/nodepool_conditions.go
--
-- Key condition types:
--   Ready                (True)  - All desired replicas are Ready nodes
--   UpdatingVersion      (True)  - A version upgrade rollout is in progress
--   UpdatingConfig       (True)  - A MachineConfig/ignition update rollout is in progress
--   ValidMachineConfig   (False) - The MachineConfig in spec.config is invalid
--   ValidReleaseImage    (False) - The release image is invalid or inaccessible
--   ValidPlatformImage   (False) - No AMI/platform image found for the release
--   ValidTuningConfig    (False) - The TuningConfig in spec.tuningConfig is invalid
--   ValidPlatformConfig  (False) - The platform-specific configuration is invalid
--   ValidMachineTemplate (False) - The generated machine template is invalid
--   ValidGeneratedPayload(False) - The ignition server could not generate a payload
--   UpdateManagementEnabled (False) - The spec.management configuration is invalid
--   SupportedVersionSkew (False) - NodePool version is outside the supported skew
--
-- ArgoCD health mapping:
--   Any Valid*/UpdateManagementEnabled/SupportedVersionSkew=False => Degraded (immediately)
--   UpdatingVersion=True or UpdatingConfig=True                   => Progressing
--   Ready=True                                                    => Healthy
--   No conditions / Ready not yet True                            => Progressing
local hs = {}

if obj.status == nil or obj.status.conditions == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for NodePool status"
  return hs
end

-- Validation failures are immediately degraded
local validationPrefixes = {
  "ValidGeneratedPayload",
  "ValidPlatformImage",
  "ValidReleaseImage",
  "ValidMachineConfig",
  "ValidTuningConfig",
  "ValidPlatformConfig",
  "ValidMachineTemplate",
  "UpdateManagementEnabled",
  "SupportedVersionSkew",
}
for _, condition in ipairs(obj.status.conditions) do
  if condition.status == "False" then
    for _, prefix in ipairs(validationPrefixes) do
      if condition.type == prefix then
        hs.status = "Degraded"
        hs.message = condition.message
        return hs
      end
    end
  end
end

-- Active version or config update
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "UpdatingVersion" and condition.status == "True" then
    hs.status = "Progressing"
    hs.message = condition.message
    return hs
  end
  if condition.type == "UpdatingConfig" and condition.status == "True" then
    hs.status = "Progressing"
    hs.message = condition.message
    return hs
  end
end

-- All replicas ready
for _, condition in ipairs(obj.status.conditions) do
  if condition.type == "Ready" and condition.status == "True" then
    hs.status = "Healthy"
    hs.message = condition.message
    return hs
  end
end

hs.status = "Progressing"
hs.message = "Waiting for NodePool to become ready"
return hs
