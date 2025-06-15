hs = {}

if obj.status == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end

-- A policy will not have a compliant field but will have a placement key set if
-- it is not being applied to any clusters
if obj.status.compliant == nil and obj.status.status == nil and obj.status.placement ~= nil and #obj.status.placement > 0 then
  hs.status = "Healthy"
  hs.message = "No clusters match this policy"
  return hs
end

if obj.status.compliant == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end

if obj.status.compliant == "Compliant" then
  hs.status = "Healthy"
else
  hs.status = "Degraded"
end

-- Collect NonCompliant clusters for the policy
noncompliants = {}
if obj.status.status ~= nil then
  -- "root" policy
  for i, entry in ipairs(obj.status.status) do
    if entry.compliant ~= "Compliant" then
      table.insert(noncompliants, entry.clustername)
    end
  end
  if #noncompliants == 0 then
    hs.message = "All clusters are compliant"
  else
    hs.message = "NonCompliant clusters: " .. table.concat(noncompliants, ", ")
  end
elseif obj.status.details ~= nil then
  -- "replicated" policy
  for i, entry in ipairs(obj.status.details) do
    if entry.compliant ~= "Compliant" then
      table.insert(noncompliants, entry.templateMeta.name)
    end
  end
  if #noncompliants == 0 then
    hs.message = "All templates are compliant"
  else
    hs.message = "NonCompliant templates: " .. table.concat(noncompliants, ", ")
  end
end

return hs
