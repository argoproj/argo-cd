-- Health check for the cert-manager CertificateRequest CRD.
-- See https://argo-cd.readthedocs.io/en/stable/operator-manual/health/ for how
-- custom health checks work and what each status means.
-- Maps the CertificateRequest status conditions to an Argo CD health status:
--   Healthy     - the Ready condition is True (certificate issued).
--   Degraded    - the request was Denied or is an InvalidRequest, or Ready is
--                 False for a reason other than Pending (e.g. Failed).
--   Progressing - Ready is False with reason Pending, or status is not populated.
local hs = {}
if obj.status ~= nil and obj.status.conditions ~= nil then
  -- A denied or invalid request is a terminal failure.
  for i, condition in ipairs(obj.status.conditions) do
    if (condition.type == "Denied" or condition.type == "InvalidRequest") and condition.status == "True" then
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
  end

  for i, condition in ipairs(obj.status.conditions) do
    if condition.type == "Ready" then
      if condition.status == "True" then
        hs.status = "Healthy"
        hs.message = condition.message
        return hs
      end
      if condition.reason == "Pending" then
        hs.status = "Progressing"
        hs.message = condition.message
        return hs
      end
      hs.status = "Degraded"
      hs.message = condition.message
      return hs
    end
  end
end

hs.status = "Progressing"
hs.message = "Waiting for certificate request"
return hs
