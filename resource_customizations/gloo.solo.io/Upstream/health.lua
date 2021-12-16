if not obj.status then
  return {status="Progressing", message="Empty status field"}
end

states={[0]=0, [1]=0, [2]=0, [3]=0}
messages={}
if obj.status.state then
  states[obj.status.state] = 1
  messages[obj.status.state] = obj.status.reason or "Unknown"
elseif obj.status.statuses then
  for k,v in pairs(obj.status.statuses or {}) do
    states[v.state] = 1
    messages[v.state] = v.reason or "Unknown"
  end
end

if states[3] > 0 then
  return {status="Degraded", message=messages[3]}
elseif states[2]  > 0 then
  return {status="Degraded", message=messages[2]}
elseif states[0] > 0 then
  return {status="Progressing", message=messages[0]}
elseif states[1] > 0 then
  return {status="Healthy", message="Healthy"}
end
return {status="Progressing", message="Unable to determine status"}
