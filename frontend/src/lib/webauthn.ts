export type CreationOptionsJSON = Omit<PublicKeyCredentialCreationOptions, 'challenge' | 'user' | 'excludeCredentials'> & {
  challenge: string;
  user: Omit<PublicKeyCredentialUserEntity, 'id'> & { id: string };
  excludeCredentials?: Array<Omit<PublicKeyCredentialDescriptor, 'id'> & { id: string }>;
};

export type RequestOptionsJSON = Omit<PublicKeyCredentialRequestOptions, 'challenge' | 'allowCredentials'> & {
  challenge: string;
  allowCredentials?: Array<Omit<PublicKeyCredentialDescriptor, 'id'> & { id: string }>;
};

export function base64urlToArrayBuffer(value: string): ArrayBuffer {
  const base64 = value.replace(/-/g, '+').replace(/_/g, '/').padEnd(Math.ceil(value.length / 4) * 4, '=');
  const binary = atob(base64);
  return Uint8Array.from(binary, (character) => character.charCodeAt(0)).buffer;
}

export function arrayBufferToBase64url(value: ArrayBuffer): string {
  const bytes = new Uint8Array(value);
  let binary = '';
  bytes.forEach((byte) => { binary += String.fromCharCode(byte); });
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function creationOptionsFromJSON(options: CreationOptionsJSON): PublicKeyCredentialCreationOptions {
  return {
    ...options,
    challenge: base64urlToArrayBuffer(options.challenge),
    user: { ...options.user, id: base64urlToArrayBuffer(options.user.id) },
    excludeCredentials: options.excludeCredentials?.map((credential) => ({
      ...credential,
      id: base64urlToArrayBuffer(credential.id),
    })),
  };
}

export function requestOptionsFromJSON(options: RequestOptionsJSON): PublicKeyCredentialRequestOptions {
  return {
    ...options,
    challenge: base64urlToArrayBuffer(options.challenge),
    allowCredentials: options.allowCredentials?.map((credential) => ({
      ...credential,
      id: base64urlToArrayBuffer(credential.id),
    })),
  };
}

export function credentialToJSON(credential: PublicKeyCredential) {
  const response = credential.response;
  const common = {
    id: credential.id,
    rawId: arrayBufferToBase64url(credential.rawId),
    type: credential.type,
    authenticatorAttachment: credential.authenticatorAttachment,
    clientExtensionResults: credential.getClientExtensionResults(),
  };

  if ('attestationObject' in response) {
    const attestation = response as AuthenticatorAttestationResponse;
    return {
      ...common,
      response: {
        clientDataJSON: arrayBufferToBase64url(attestation.clientDataJSON),
        attestationObject: arrayBufferToBase64url(attestation.attestationObject),
        transports: attestation.getTransports?.(),
      },
    };
  }

  const assertion = response as AuthenticatorAssertionResponse;
  return {
    ...common,
    response: {
      clientDataJSON: arrayBufferToBase64url(assertion.clientDataJSON),
      authenticatorData: arrayBufferToBase64url(assertion.authenticatorData),
      signature: arrayBufferToBase64url(assertion.signature),
      userHandle: assertion.userHandle ? arrayBufferToBase64url(assertion.userHandle) : null,
    },
  };
}

export const isWebAuthnSupported = () =>
  typeof window !== 'undefined' && 'PublicKeyCredential' in window && !!navigator.credentials;

export const isWebAuthnCancellation = (error: unknown) =>
  error instanceof DOMException && (error.name === 'NotAllowedError' || error.name === 'AbortError');
