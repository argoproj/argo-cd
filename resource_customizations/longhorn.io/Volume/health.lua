-- Health check for the Longhorn Volume CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the Volume status.state and status.robustness to an Argo CD health status:
--   Degraded    - robustness is "faulted" (the volume's data is unavailable).
--   Progressing - the volume is being created/attached/detached, or is attached
--                 but rebuilding replicas ("degraded" robustness).
--   Healthy     - the volume is attached and healthy, or cleanly detached (idle).
local hs = { status = "Progressing", message = "Waiting for volume" }
if obj.status ~= nil then
  local state = obj.status.state
  local robustness = obj.status.robustness

  if robustness == "faulted" then
    hs.status = "Degraded"
    hs.message = "Volume is faulted"
    return hs
  end

  if state == "attached" then
    if robustness == "healthy" then
      hs.status = "Healthy"
      hs.message = "Volume is attached and healthy"
    elseif robustness == "degraded" then
      hs.status = "Progressing"
      hs.message = "Volume is degraded (rebuilding replicas)"
    else
      hs.status = "Progressing"
      hs.message = "Volume is attached"
    end
    return hs
  elseif state == "detached" then
    hs.status = "Healthy"
    hs.message = "Volume is detached"
    return hs
  elseif state ~= nil and state ~= "" then
    hs.status = "Progressing"
    hs.message = "Volume is " .. state
    return hs
  end
end
return hs
