import {
    AuthorizationServer,
    Client,
    authorizationCodeGrantRequest,
    calculatePKCECodeChallenge,
    discoveryRequest,
    expectNoState,
    generateRandomCodeVerifier,
    isOAuth2Error,
    parseWwwAuthenticateChallenges,
    processAuthorizationCodeOpenIDResponse,
    processDiscoveryResponse,
    validateAuthResponse
} from 'oauth4webapi';
import {AuthSettings} from '../../shared/models';

export const discoverAuthServer = (issuerURL: URL): Promise<AuthorizationServer> => discoveryRequest(issuerURL).then(res => processDiscoveryResponse(issuerURL, res));

export const PKCECodeVerifier = {
    get: () => sessionStorage.getItem(window.btoa('code_verifier')),
    set: (codeVerifier: string) => sessionStorage.setItem(window.btoa('code_verifier'), codeVerifier),
    unset: () => sessionStorage.removeItem(window.btoa('code_verifier'))
};

export const getPKCERedirectURI = () => {
    const currentOrigin = new URL(window.location.origin);

    currentOrigin.pathname = '/pkce/verify';

    return currentOrigin;
};

export class PKCELoginError extends Error {
    constructor(message: string) {
        super(message);
        this.name = 'PKCELoginError';
    }
}

const validateAndGetOIDCForPKCE = async (oidcConfig: AuthSettings['oidcConfig']) => {
    if (!oidcConfig) {
        throw new PKCELoginError('No OIDC Config found');
    }

    let issuerURL: URL;
    try {
        issuerURL = new URL(oidcConfig.issuer);
    } catch (e) {
        throw new PKCELoginError(`Invalid oidc issuer ${oidcConfig.issuer}`);
    }

    if (!oidcConfig.clientID) {
        throw new PKCELoginError('No OIDC Client Id found');
    }

    let authorizationServer: AuthorizationServer;
    try {
        authorizationServer = await discoverAuthServer(issuerURL);
    } catch (e) {
        throw new PKCELoginError(e);
    }

    return {
        issuerURL,
        authorizationServer,
        clientID: oidcConfig.clientID
    };
};

export const pkceLogin = async (oidcConfig: AuthSettings['oidcConfig'], redirectURI: string) => {
    const {authorizationServer} = await validateAndGetOIDCForPKCE(oidcConfig);

    if (!authorizationServer.authorization_endpoint) {
        throw new PKCELoginError('No Authorization Server endpoint found');
    }

    const codeVerifier = generateRandomCodeVerifier();

    const codeChallange = await calculatePKCECodeChallenge(codeVerifier);

    const authorizationServerConsentScreen = new URL(authorizationServer.authorization_endpoint);

    authorizationServerConsentScreen.searchParams.set('client_id', oidcConfig.clientID);
    authorizationServerConsentScreen.searchParams.set('code_challenge', codeChallange);
    authorizationServerConsentScreen.searchParams.set('code_challenge_method', 'S256');
    authorizationServerConsentScreen.searchParams.set('redirect_uri', redirectURI);
    authorizationServerConsentScreen.searchParams.set('response_type', 'code');
    authorizationServerConsentScreen.searchParams.set('scope', oidcConfig.scopes.join(' '));

    PKCECodeVerifier.set(codeVerifier);

    window.location.replace(authorizationServerConsentScreen.toString());
};

export const pkceCallback = async (queryParams: string, oidcConfig: AuthSettings['oidcConfig'], redirectURI: string) => {
    const codeVerifier = PKCECodeVerifier.get();

    if (!codeVerifier) {
        throw new PKCELoginError('No code verifier found in session');
    }

    let callbackQueryParams = new URLSearchParams();
    try {
        callbackQueryParams = new URLSearchParams(queryParams);
    } catch (e) {
        throw new PKCELoginError('Invalid query parameters');
    }

    if (!callbackQueryParams.get('code')) {
        throw new PKCELoginError('No code in query parameters');
    }

    if (callbackQueryParams.get('state') === '') {
        callbackQueryParams.delete('state');
    }

    const {authorizationServer} = await validateAndGetOIDCForPKCE(oidcConfig);

    const client: Client = {
        client_id: oidcConfig.clientID,
        token_endpoint_auth_method: 'none'
    };

    const params = validateAuthResponse(authorizationServer, client, callbackQueryParams, expectNoState);

    if (isOAuth2Error(params)) {
        throw new PKCELoginError('Error validating auth response');
    }

    const response = await authorizationCodeGrantRequest(authorizationServer, client, params, redirectURI, codeVerifier);

    const authChallengeExtract = parseWwwAuthenticateChallenges(response);

    if (authChallengeExtract?.length > 0) {
        throw new PKCELoginError('Error parsing authentication challenge');
    }

    const result = await processAuthorizationCodeOpenIDResponse(authorizationServer, client, response);

    if (isOAuth2Error(result)) {
        throw new PKCELoginError(`Error getting token ${result.error_description}`);
    }

    if (!result.id_token) {
        throw new PKCELoginError('No token in response');
    }

    document.cookie = `argocd.token=${result.id_token}; path=/`;

    window.location.replace('/applications');
};
