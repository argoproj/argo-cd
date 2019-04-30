if obj.status.verifyingPreview ~= nil and obj.status.verifyingPreview then
    obj.status.verifyingPreview = false
end

if obj.spec.paused ~= nil and obj.spec.paused then
    obj.spec.paused = false
end

return obj