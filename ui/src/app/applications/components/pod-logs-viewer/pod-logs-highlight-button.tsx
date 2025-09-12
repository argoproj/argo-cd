import * as React from 'react';
import {useState, useRef, useEffect} from 'react';
import {ToggleButton} from '../../../shared/components/toggle-button';

// PodHighlightButton is a component that renders a toggle button that toggles pod highlighting.
export const PodHighlightButton = ({
    selectedPod,
    setSelectedPod,
    pods,
    darkMode
}: {
    selectedPod: string | null;
    setSelectedPod: (value: string | null) => void;
    pods: string[];
    darkMode: boolean;
}) => {
    const [isOpen, setIsOpen] = useState(false);
    const dropdownRef = useRef<HTMLDivElement>(null);

    useEffect(() => {
        const handleClickOutside = (event: MouseEvent) => {
            if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
                setIsOpen(false);
            }
        };

        document.addEventListener('mousedown', handleClickOutside);
        return () => document.removeEventListener('mousedown', handleClickOutside);
    }, []);

    return (
        <div ref={dropdownRef} style={{position: 'relative'}}>
            <ToggleButton title='Select a pod to highlight its logs' onToggle={() => setIsOpen(!isOpen)} icon='highlighter' toggled={selectedPod !== null} />
            {isOpen && (
                <div className={`select-container ${darkMode ? 'dark-mode' : ''}`} style={{position: 'absolute', top: '100%', left: 0, zIndex: 1}}>
                    <div className='select-options'>
                        {pods.map(pod => (
                            <div
                                key={pod}
                                className={`select-option ${selectedPod === pod ? 'selected' : ''} ${darkMode ? 'dark-mode' : ''}`}
                                onClick={() => {
                                    setSelectedPod(pod);
                                    setIsOpen(false);
                                }}>
                                {pod}
                            </div>
                        ))}
                    </div>
                    <div
                        className={`select-option clear-highlight ${darkMode ? 'dark-mode' : ''}`}
                        onClick={() => {
                            setSelectedPod(null);
                            setIsOpen(false);
                        }}>
                        Clear highlight
                    </div>
                </div>
            )}
        </div>
    );
};
