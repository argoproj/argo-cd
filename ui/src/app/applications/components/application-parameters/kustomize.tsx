import {Checkbox} from 'argo-ui';
import * as React from 'react';
import {FieldApi, FormField as ReactFormField} from 'react-form';

import {format, parse} from './kustomize-image';

export const ImageTagFieldEditor = ReactFormField((props: {metadata: {value: string}; fieldApi: FieldApi; className: string}) => {
    const {
        fieldApi: {getValue, setValue}
    } = props;
    const origImage = parse(props.metadata.value);
    const val = getValue();
    const image = val ? parse(val) : {name: origImage.name};
    const mustBeDigest = (image.digest || '').indexOf(':') > -1;
    return (
        <div>
            <input
                style={{width: 'calc(50% - 1em)', marginRight: '1em'}}
                placeholder={origImage.name}
                className={props.className}
                value={image.newName || ''}
                onChange={el => {
                    setValue(format({...image, newName: el.target.value}));
                }}
            />
            <input
                style={{width: 'calc(50% - 12em)'}}
                className={props.className}
                onChange={el => {
                    const forceDigest = el.target.value.indexOf(':') > -1;
                    if (image.digest || forceDigest) {
                        setValue(format({...image, newTag: null, digest: el.target.value}));
                    } else {
                        setValue(format({...image, newTag: el.target.value, digest: null}));
                    }
                }}
                placeholder={origImage.newTag || origImage.digest}
                value={image.newTag || image.digest || ''}
            />
            <div style={{width: '6em', display: 'inline-block'}}>
                <Checkbox
                    checked={!!image.digest}
                    id={`${image.name}_is-digest`}
                    onChange={() => {
                        const nextImg = {...image};
                        if (mustBeDigest) {
                            return;
                        }
                        if (nextImg.digest) {
                            nextImg.newTag = nextImg.digest;
                            nextImg.digest = null;
                        } else {
                            nextImg.digest = nextImg.newTag;
                            nextImg.newTag = null;
                        }
                        setValue(format(nextImg));
                    }}
                />{' '}
                <label htmlFor={`${image.name}_is-digest`}> Digest?</label>
            </div>
        </div>
    );
});
