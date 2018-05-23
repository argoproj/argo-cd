import * as classNames from 'classnames';
import * as React from 'react';

export interface PopupProps extends React.Props<any> {
    icon?: { name: string; color: string; };
    title: string | React.ReactNode;
    content?: React.ReactNode;
    footer?: React.ReactNode;
}

require('./popup.scss');

export const Popup = (props: PopupProps) => (
    <div className='popup-overlay'>
        <div className='popup-container'>
            <div className='row popup-container__header'>
                {props.title}
            </div>
            <div className='row popup-container__body'>
                {props.icon &&
                    <div className='columns large-2 popup-container__icon'>
                        <i className={`${props.icon.name} ${props.icon.color}`}/>
                    </div>
                }
                <div className={classNames('columns', {'large-10': !!props.icon, 'large-12': !props.icon})}>
                    {props.content}
                </div>
            </div>

            <div className={classNames('row popup-container__footer', {'popup-container__footer--additional-padding': !!props.icon})}>
                {props.footer}
            </div>
        </div>
    </div>
);
