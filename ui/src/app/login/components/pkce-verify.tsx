import React, {useEffect, useState} from 'react';
import {RouteComponentProps} from 'react-router';
import {services} from '../../shared/services';
import {PKCECodeVerifier, PKCEState, PKCELoginError, getPKCERedirectURI, pkceCallback} from './utils';

import './pkce-verify.scss';

export const PKCEVerification = (props: RouteComponentProps<any>) => {
    const [loading, setLoading] = useState(true);
    const [error, setError] = useState<PKCELoginError | Error>();

    useEffect(() => {
        setLoading(true);
        services.authService
            .settings()
            .then(authSettings => pkceCallback(props.location.search, authSettings.oidcConfig, getPKCERedirectURI().toString()))
            .catch(err => setError(err))
            .finally(() => {
                setLoading(false);
                PKCECodeVerifier.unset();
                PKCEState.unset();
            });
    }, [props.location]);

    if (loading) {
        return <div className='pkce-verify__container'>Processing...</div>;
    }

    if (error) {
        return (
            <div className='pkce-verify__container'>
                <div>
                    <h3>Error occurred: </h3>
                    <p>{error?.message || JSON.stringify(error)}</p>
                    <a href='/login'>Try to Login again</a>
                </div>
            </div>
        );
    }

    return (
        <div className='pkce-verify__container'>
            success. if you are not being redirected automatically please &nbsp;<a href='/applications'>click here</a>
        </div>
    );
};
