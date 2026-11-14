-- Health check for the cert-manager ACME Challenge CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the ACME Challenge status.state to an Argo CD health status:
--   Healthy     - the challenge is valid (it has been satisfied).
--   Degraded    - the challenge is invalid, expired, or errored.
--   Progressing - the challenge is pending, ready, or processing, or status is
--                 not populated yet.
local hs = { status = "Progressing", message = "Waiting for challenge" }
if obj.status ~= nil and obj.status.state ~= nil and obj.status.state ~= "" then
  local state = obj.status.state
  if state == "valid" then
    hs.status = "Healthy"
    hs.message = "Challenge is valid"
  elseif state == "invalid" or state == "expired" or state == "errored" then
    hs.status = "Degraded"
    hs.message = obj.status.reason or ("Challenge is " .. state)
  else
    hs.status = "Progressing"
    hs.message = obj.status.reason or ("Challenge is " .. state)
  end
end
return hs
