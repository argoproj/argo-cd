import * as React from 'react';

interface DisableSyncPopupProps {
    defaultMinutes: number;
    onDateChange: (isoDate: string) => void;
}

export function DisableSyncPopup({defaultMinutes, onDateChange}: DisableSyncPopupProps) {
    const [selectedDate, setSelectedDate] = React.useState(() => new Date(Date.now() + defaultMinutes * 60000));

    React.useEffect(() => {
        onDateChange(selectedDate.toISOString());
    }, [selectedDate]);

    const applyPreset = (minutes: number) => setSelectedDate(new Date(Date.now() + minutes * 60000));

    const toLocal = (d: Date) => {
        const pad = (n: number) => String(n).padStart(2, '0');
        return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`;
    };

    return (
        <div style={{marginTop: '20px', marginBottom: '20px'}}>
            <div style={{marginBottom: '10px'}}>
                <button className='argo-button argo-button--base-o' style={{marginRight: '8px'}} onClick={() => applyPreset(15)}>
                    15 min
                </button>
                <button className='argo-button argo-button--base-o' style={{marginRight: '8px'}} onClick={() => applyPreset(60)}>
                    1 hour
                </button>
                <button className='argo-button argo-button--base-o' style={{marginRight: '8px'}} onClick={() => applyPreset(480)}>
                    8 hours
                </button>
                <button className='argo-button argo-button--base-o' style={{marginRight: '8px'}} onClick={() => applyPreset(1440)}>
                    24 hours
                </button>
            </div>
            <div>
                <input
                    type='datetime-local'
                    className='argo-field'
                    value={toLocal(selectedDate)}
                    min={toLocal(new Date())}
                    onChange={e => e.target.value && setSelectedDate(new Date(e.target.value))}
                    style={{width: '50%'}}
                />
            </div>
        </div>
    );
}
