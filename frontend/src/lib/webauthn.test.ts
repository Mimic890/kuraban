import { describe, expect, it } from 'vitest';
import {
  arrayBufferToBase64url,
  base64urlToArrayBuffer,
  creationOptionsFromJSON,
  credentialToJSON,
  requestOptionsFromJSON,
  type CreationOptionsJSON,
  type RequestOptionsJSON,
} from './webauthn';

const bytes = (...values: number[]) => new Uint8Array(values).buffer;
const values = (source: BufferSource) => [...new Uint8Array(source as ArrayBuffer)];

describe('WebAuthn JSON conversion', () => {
  it('round-trips unpadded base64url values', () => {
    const encoded = arrayBufferToBase64url(bytes(1, 2, 3, 254, 255));
    expect(encoded).toBe('AQID_v8');
    expect([...new Uint8Array(base64urlToArrayBuffer(encoded))]).toEqual([1, 2, 3, 254, 255]);
  });

  it('decodes creation and request option binary fields', () => {
    const creation = creationOptionsFromJSON({
      challenge: 'AQI',
      rp: { name: 'Garage' },
      user: { id: 'AwQ', name: 'admin', displayName: 'Admin' },
      pubKeyCredParams: [{ type: 'public-key', alg: -7 }],
      excludeCredentials: [{ type: 'public-key', id: 'BQY' }],
    } as CreationOptionsJSON);
    const request = requestOptionsFromJSON({
      challenge: 'Bwg',
      allowCredentials: [{ type: 'public-key', id: 'CQo' }],
    } as RequestOptionsJSON);

    expect(values(creation.challenge)).toEqual([1, 2]);
    expect(values(creation.user.id)).toEqual([3, 4]);
    expect(values(creation.excludeCredentials![0].id)).toEqual([5, 6]);
    expect(values(request.challenge)).toEqual([7, 8]);
    expect(values(request.allowCredentials![0].id)).toEqual([9, 10]);
  });

  it('serializes an assertion without retaining credential objects', () => {
    const credential = {
      id: 'credential-id',
      rawId: bytes(1),
      type: 'public-key',
      authenticatorAttachment: 'platform',
      getClientExtensionResults: () => ({ appid: false }),
      response: {
        clientDataJSON: bytes(2),
        authenticatorData: bytes(3),
        signature: bytes(4),
        userHandle: null,
      },
    } as unknown as PublicKeyCredential;

    expect(credentialToJSON(credential)).toEqual({
      id: 'credential-id',
      rawId: 'AQ',
      type: 'public-key',
      authenticatorAttachment: 'platform',
      clientExtensionResults: { appid: false },
      response: {
        clientDataJSON: 'Ag',
        authenticatorData: 'Aw',
        signature: 'BA',
        userHandle: null,
      },
    });
  });
});
