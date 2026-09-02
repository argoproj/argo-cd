local hs = {
  status = "Progressing",
  message = "Waiting for CloudNativePG to reconcile Database"
}

if obj.status == nil then
  return hs
end

-- CloudNativePG only advances status.observedGeneration on a *successful*
-- reconcile: SetAsFailed sets applied=false and a message but leaves
-- observedGeneration untouched, whereas SetAsReady sets applied=true and
-- advances observedGeneration to the current generation. See database_funcs.go:
-- https://github.com/cloudnative-pg/cloudnative-pg/blob/v1.30.0/api/v1/database_funcs.go
-- A failed reconcile must therefore be evaluated before the observedGeneration
-- guard below, otherwise a permanently failing Database would report
-- Progressing forever instead of Degraded.
if obj.status.applied == false then
  hs.status = "Degraded"
  hs.message = obj.status.message or "CloudNativePG failed to apply the Database"
  return hs
end

-- A new spec generation that CNPG has not observed yet: keep the previous
-- successful state from blocking, but do not report Healthy until the current
-- generation has actually been applied.
if obj.metadata ~= nil and obj.metadata.generation ~= nil and
   obj.status.observedGeneration ~= obj.metadata.generation then
  hs.message = "Waiting for CloudNativePG to observe the current Database generation"
  return hs
end

if obj.status.applied == true then
  hs.status = "Healthy"
  hs.message = obj.status.message or ""
  return hs
end

if obj.status.message ~= nil then
  hs.message = obj.status.message
end
return hs
