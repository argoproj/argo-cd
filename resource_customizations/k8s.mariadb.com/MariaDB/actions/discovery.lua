local actions = {}

local isSuspended = false
if obj.spec and obj.spec.suspend and obj.spec.suspend == true then
	isSuspended = true
end

if isSuspended then
	actions["resume"] = {
		["iconClass"] = "fa fa-fw fa-play",
		["displayName"] = "Resume"
	}
else
	actions["suspend"] = {
		["iconClass"] = "fa fa-fw fa-pause",
		["displayName"] = "Suspend"
	}
end

return actions
