-- ImageValidatingPolicy is a cluster-scoped resource that defines CEL-based
-- image signature and attestation validation rules applied to container images
-- during admission via a Kyverno webhook.
--
-- Documentation:
--   Policy types overview: https://kyverno.io/docs/policy-types/image-validating-policy/
--
-- Condition types and reasons are defined in:
--   https://github.com/kyverno/kyverno/tree/main/config/crds/policies.kyverno.io/policies.kyverno.io_imagevalidatingpolicies.yaml
--
-- ImageValidatingPolicy exposes a conditionStatus with a ready boolean and
-- standard Kubernetes conditions, including:
--   WebhookConfigured (True)  - Kyverno webhook is configured for the policy
--   WebhookConfigured (False) - Webhook configuration failed
--
-- ArgoCD health mapping:
--   conditionStatus.ready=true   => Healthy  (WebhookConfigured condition message)
--   conditionStatus.ready=false  => Degraded (message from first False condition)
--   No status yet                => Progressing
local hs = {}

if obj.status ~= nil and obj.status.conditionStatus ~= nil then
  local cs = obj.status.conditionStatus

  if cs.ready == true then
    hs.status = "Healthy"
    if cs.conditions ~= nil then
      for _, condition in ipairs(cs.conditions) do
        if condition.type == "WebhookConfigured" and condition.status == "True" then
          hs.message = condition.message
          break
        end
      end
    end
    if hs.message == nil then
      hs.message = (cs.message ~= nil and cs.message ~= "") and cs.message or "Policy is ready"
    end
    return hs
  end

  hs.status = "Degraded"
  if cs.conditions ~= nil then
    for _, condition in ipairs(cs.conditions) do
      if condition.status == "False" then
        hs.message = condition.type .. ": " .. condition.message
        return hs
      end
    end
  end
  hs.message = (cs.message ~= nil and cs.message ~= "") and cs.message or "Policy is not ready"
  return hs
end

hs.status = "Progressing"
hs.message = "Waiting for ImageValidatingPolicy status"
return hs
