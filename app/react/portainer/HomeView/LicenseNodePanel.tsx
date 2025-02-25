import { LicenseType } from '@/portainer/license-management/types';
import { useLicenseInfo } from '@/portainer/license-management/use-license.service';

import { TextTip } from '@@/Tip/TextTip';
import { InformationPanel } from '@@/InformationPanel';

import { useNodesCount } from '../system/useNodesCount';

export function LicenseNodePanel() {
  const nodesValid = useNodesValid();

  if (nodesValid) {
    return null;
  }

  return (
    <InformationPanel title="License node allowance exceeded">
      <TextTip>
        The number of nodes for your license has been exceeded. Please contact
        your administrator.
      </TextTip>
    </InformationPanel>
  );
}

function useNodesValid() {
  const { isLoading: isLoadingNodes, data: nodesCount = 0 } = useNodesCount();

  const { isLoading: isLoadingLicense, info } = useLicenseInfo();
  if (
    isLoadingLicense ||
    isLoadingNodes ||
    !info ||
    info.type === LicenseType.Trial
  ) {
    return true;
  }

  return nodesCount <= info.nodes;
}
