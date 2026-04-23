-- GatewayClass health check for Argo CD
--
-- GatewayClass is a cluster-scoped Gateway API resource whose readiness is
-- expressed through .status.conditions[type=Accepted].
-- A Gateway API controller watches GatewayClass resources and sets the
-- Accepted condition to indicate whether it can serve Gateways for this class.
--
-- Accepted condition semantics (Gateway API spec):
--   True    – controller accepts this class and can provision Gateways
--   False   – controller rejects this class (e.g. invalid parameters)
--   Unknown – controller has not yet reconciled (default initial state, reason=Pending)
--
-- Argo CD health mapping:
--   Accepted=True              -> Healthy
--   Accepted=False             -> Degraded
--   Accepted=Unknown / absent  -> Progressing
--   observedGeneration stale   -> Progressing (avoids reporting outdated status)
--
-- References:
--   GatewayClass status:        https://gateway-api.sigs.k8s.io/api-types/gatewayclass/#gatewayclass-status
--   Accepted condition type:    https://gateway-api.sigs.k8s.io/reference/spec/#gateway.networking.k8s.io/v1.GatewayClassConditionType
--   observedGeneration pattern: https://gateway-api.sigs.k8s.io/guides/implementers/#conditions

local hs = {}

if obj.status ~= nil and obj.status.conditions ~= nil then
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Accepted" then
      -- Skip stale conditions where the controller has not yet reconciled the current generation.
      -- See: https://gateway-api.sigs.k8s.io/guides/implementers/#conditions
      if obj.metadata ~= nil and obj.metadata.generation ~= nil and condition.observedGeneration ~= nil then
        if condition.observedGeneration ~= obj.metadata.generation then
          goto continue
        end
      end

      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message or "GatewayClass is accepted"
        return hs
      elseif condition.status == "False" then
        hs.status = "Degraded"
        hs.message = condition.message or "GatewayClass is not accepted"
        return hs
      else
        hs.status = "Progressing"
        hs.message = condition.message or "Waiting for GatewayClass status"
        return hs
      end
    end
    ::continue::
  end
end

hs.status = "Progressing"
hs.message = "Waiting for GatewayClass status"
return hs
