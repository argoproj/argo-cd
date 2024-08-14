hs = {}
if obj.status == nil or obj.status.compliant == nil then
  hs.status = "Progressing"
  hs.message = "Waiting for the status to be reported"
  return hs
end
if obj.status.compliant == "Compliant" then
  hs.status = "Healthy"
else
  hs.status = "Degraded"
end
noncompliants = {}
if obj.status.status ~= nil then
  -- "root" policy
  for i, entry in ipairs(obj.status.status) do
    if entry.compliant ~= "Compliant" then
      noncompliants[i] = entry.clustername
    end
  end
  if table.getn(noncompliants) == 0 then
    hs.message = "All clusters are compliant"
  else
    hs.message = "NonCompliant clusters: " .. table.concat(noncompliants, ", ")
  end
elseif obj.status.details ~= nil then
  -- "replicated" policy
  for i, entry in ipairs(obj.status.details) do
    if entry.compliant ~= "Compliant" then
      noncompliants[i] = entry.templateMeta.name
    end
  end
  if table.getn(noncompliants) == 0 then
    hs.message = "All templates are compliant"
  else
    hs.message = "NonCompliant templates: " .. table.concat(noncompliants, ", ")
  end
end
return hs
