import * as React from 'react';
import { BehaviorSubject, Observable } from 'rxjs';
import { PopupProps  } from './popup';

export interface PopupApi {
    confirm(title: string, message: string | React.ReactNode): Promise<boolean>;
}

export class PopupManager implements PopupApi {
    private popupPropsSubject: BehaviorSubject<PopupProps> = new BehaviorSubject(null);

    public get popupProps(): Observable<PopupProps> {
        return this.popupPropsSubject.asObservable();
    }

    public confirm(title: string, message: string): Promise<boolean> {
        return new Promise((resolve) => {
            const closeAndResolve = (result: boolean) => {
                this.popupPropsSubject.next(null);
                resolve(result);
            };

            this.popupPropsSubject.next({
                title: (
                    <span>{title} <i className='argo-icon-close' onClick={() => closeAndResolve(false)}/></span>
                ),
                content: (
                    <div>
                        <p>{message}</p>
                    </div>
                ),
                footer: (
                    <div>
                        <button className='argo-button argo-button--base' onClick={() => closeAndResolve(true)}>Yes</button> <button
                            className='argo-button argo-button--base-o' onClick={() => closeAndResolve(false)}>Cancel</button>
                    </div>
                ),
            });
        });
    }
}
