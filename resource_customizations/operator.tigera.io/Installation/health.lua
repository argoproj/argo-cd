-- Health check for the Calico (Tigera operator) Installation CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- The Installation reports component rollout via standard status conditions:
--   Degraded    - the Degraded condition is True.
--   Healthy     - the Available condition is True and up to date with the spec.
--   Progressing - the status has not yet observed the current generation, or
--                 otherwise (rolling out, or status not populated yet).
local hs = { status = "Progressing", message = "Waiting for Installation to become available" }
if obj.status ~= nil and obj.status.conditions ~= nil then
  local generation = nil
  if obj.metadata ~= nil then
    generation = obj.metadata.generation
  end

  -- Degraded takes precedence over everything else.
  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Degraded" and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message or condition.reason or "Installation is degraded"
      return hs
    end
  end

  for _, condition in ipairs(obj.status.conditions) do
    if condition.type == "Available" and condition.status == "True" then
      -- If the condition observed an older generation than the current spec,
      -- the operator has not finished reconciling the latest changes yet.
      if generation ~= nil and condition.observedGeneration ~= nil and condition.observedGeneration ~= generation then
        hs.status = "Progressing"
        hs.message = "Waiting for Installation status to observe the current generation"
        return hs
      end
      hs.status = "Healthy"
      hs.message = condition.message or "All Calico components are available"
      return hs
    end
  end
end
return hs
