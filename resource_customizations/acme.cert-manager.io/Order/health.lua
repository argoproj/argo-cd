-- Health check for the cert-manager ACME Order CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the ACME Order status.state to an Argo CD health status:
--   Healthy     - the order is valid (the certificate has been issued).
--   Degraded    - the order is invalid, expired, or errored.
--   Progressing - the order is pending, ready, or processing, or status is
--                 not populated yet.
local hs = { status = "Progressing", message = "Waiting for order" }
if obj.status ~= nil and obj.status.state ~= nil and obj.status.state ~= "" then
  local state = obj.status.state
  if state == "valid" then
    hs.status = "Healthy"
    hs.message = "Order is valid"
  elseif state == "invalid" or state == "expired" or state == "errored" then
    hs.status = "Degraded"
    hs.message = obj.status.reason or ("Order is " .. state)
  else
    hs.status = "Progressing"
    hs.message = "Order is " .. state
  end
end
return hs
