import { Heading } from '../account/shared';
import MarketplacePanel from './MarketplacePanel';

export default function MarketplacePage() {
  return (
    <>
      <Heading title="市场管理">
        创建技能、组合装备组，或从 GitHub 同步开源技能。
      </Heading>
      <MarketplacePanel />
    </>
  );
}
