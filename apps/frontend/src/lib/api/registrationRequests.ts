import { AdminRegistrationRequestService } from '@chatto/api-types/admin/v1/registration_requests_connect';
import type { PendingRegistration as APIPendingRegistration } from '@chatto/api-types/admin/v1/registration_requests_pb';
import { createChattoClient, type ConnectAPIConfig } from '@chatto/client/api/connect';

/** A pending self-service account request that an administrator can approve or deny. */
export type RegistrationRequest = {
  id: string;
  login: string;
  email: string;
  note: string;
  createdAt: string;
};

export function createRegistrationRequestAPI(config: ConnectAPIConfig) {
  const client = createChattoClient(AdminRegistrationRequestService, config);
  return {
    async list(offset = 0, limit = 20, options: { signal?: AbortSignal } = {}) {
      const response = await client.listRegistrationRequests(
        { page: { offset, limit } },
        { signal: options.signal }
      );
      return {
        requests: response.registrationRequests.map(mapRegistrationRequest),
        totalCount: Number(response.page?.totalCount ?? 0),
        hasMore: response.page?.hasMore ?? false
      };
    },
    /** Creates the account. Resolves to the login that the account now uses. */
    async approve(id: string) {
      const response = await client.approveRegistrationRequest({ id });
      return response.login;
    },
    async deny(id: string) {
      await client.denyRegistrationRequest({ id });
    }
  };
}

function mapRegistrationRequest(request: APIPendingRegistration): RegistrationRequest {
  return {
    id: request.id,
    login: request.login,
    email: request.email,
    note: request.note,
    createdAt: request.createdAt ? request.createdAt.toDate().toISOString() : ''
  };
}
