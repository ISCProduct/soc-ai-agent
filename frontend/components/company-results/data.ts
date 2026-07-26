import type { Company, UserData, UserScore } from './types'

export const getBaseCompanyData = (): Company[] => {
  return [
    {
      id: 1,
      name: "株式会社テックイノベーション",
      industry: "Webサービス・AI開発",
      location: "東京都渋谷区",
      employees: "150名",
      size: "ベンチャー企業（100-300名）",
      description: "自社AIプロダクトを開発するベンチャー企業。最新技術を活用した開発環境で急成長中。機械学習エンジニアとして最前線で活躍できる環境を提供しています。",
      matchScore: 0,
      tags: ["リモートワーク", "フレックス", "技術力重視"],
      techStack: ["Python", "TypeScript", "React", "AWS"],
      projectTypes: ["自社サービス", "AI開発"],
      salary: "400-650万円",
      benefits: ["リモートワーク可", "フレックスタイム", "書籍購入補助", "資格取得支援", "最新機器貸与"],
      culture: ["技術第一", "フラットな組織", "挑戦を推奨", "成果主義"],
      founded: "2018年",
      website: "https://tech-innovation.example.com",
      parentCompany: undefined,
      subsidiaries: ["TI Lab株式会社", "TI Consulting"],
      partnerships: ["Google Cloud", "AWS", "Microsoft Azure"],
      capitalStructure: {
        shareholders: [
          { name: "創業者グループ", percentage: 45 },
          { name: "ベンチャーキャピタルA", percentage: 30 },
          { name: "事業会社B", percentage: 15 },
          { name: "従業員持株会", percentage: 10 },
        ],
      },
    },
    {
      id: 2,
      name: "日本システムソリューションズ株式会社",
      industry: "SIer・受託開発",
      location: "東京都千代田区",
      employees: "2500名",
      size: "大手企業（1000名以上）",
      description: "大手企業向けシステム開発を手がける老舗SIer。充実した研修制度と安定した環境で、エンジニアとしての基礎をしっかり学べます。",
      matchScore: 0,
      tags: ["大手企業", "研修充実", "福利厚生"],
      techStack: ["Java", "Oracle", "Spring"],
      projectTypes: ["受託開発", "社内システム"],
      salary: "350-550万円",
      benefits: ["住宅手当", "家族手当", "退職金制度", "社員食堂", "保養所"],
      culture: ["安定重視", "チームワーク", "長期キャリア形成", "教育重視"],
      founded: "1985年",
      website: "https://nss.example.co.jp",
      parentCompany: "日本テクノロジーグループ",
      subsidiaries: ["NSS北海道", "NSS関西", "NSS九州"],
      partnerships: ["Oracle", "SAP", "IBM"],
      capitalStructure: {
        shareholders: [
          { name: "日本テクノロジーグループ", percentage: 60 },
          { name: "機関投資家", percentage: 25 },
          { name: "自社株", percentage: 10 },
          { name: "その他", percentage: 5 },
        ],
      },
    },
    {
      id: 3,
      name: "クラウドテック株式会社",
      industry: "クラウド・インフラ",
      location: "東京都港区",
      employees: "300名",
      size: "中堅企業（300-800名）",
      description: "クラウドインフラの設計・構築を専門とする企業。AWS/Azure/GCPの認定資格取得支援あり。",
      matchScore: 0,
      tags: ["インフラ特化", "資格支援", "技術研修"],
      techStack: ["AWS", "Kubernetes", "Terraform"],
      projectTypes: ["インフラ構築", "クラウド移行"],
      salary: "550-750万円",
      benefits: ["資格取得支援", "リモートワーク可", "技術書籍購入補助", "認定資格手当"],
      culture: ["専門性重視", "技術研修充実", "実力主義", "自己成長推奨"],
      founded: "2016年",
      website: "https://cloud-tech.example.com",
      parentCompany: undefined,
      subsidiaries: [],
      partnerships: ["AWS", "Microsoft Azure", "Google Cloud"],
      capitalStructure: {
        shareholders: [
          { name: "創業者", percentage: 55 },
          { name: "事業会社C", percentage: 30 },
          { name: "従業員持株会", percentage: 15 },
        ],
      },
    },
    {
      id: 4,
      name: "株式会社セキュアネット",
      industry: "セキュリティ・コンサルティング",
      location: "東京都新宿区",
      employees: "180名",
      size: "ベンチャー企業（100-300名）",
      description: "サイバーセキュリティのスペシャリスト集団。高度な技術力と専門性を磨ける環境。",
      matchScore: 0,
      tags: ["セキュリティ", "高給与", "専門性"],
      techStack: ["Python", "Linux", "ネットワーク", "Wireshark"],
      projectTypes: ["セキュリティ診断", "コンサルティング"],
      salary: "500-800万円",
      benefits: ["高給与", "資格取得支援", "技術カンファレンス参加費補助", "セキュリティ書籍購入"],
      culture: ["専門性追求", "技術力重視", "継続学習", "セキュリティファースト"],
      founded: "2015年",
      website: "https://securenet.example.com",
      parentCompany: undefined,
      subsidiaries: [],
      partnerships: ["IPA", "JPCERT/CC"],
      capitalStructure: {
        shareholders: [
          { name: "創業者グループ", percentage: 70 },
          { name: "エンジェル投資家", percentage: 20 },
          { name: "従業員持株会", percentage: 10 },
        ],
      },
    },
    {
      id: 5,
      name: "データアナリティクス株式会社",
      industry: "データ分析・BI",
      location: "東京都品川区",
      employees: "120名",
      size: "ベンチャー企業（100-300名）",
      description: "ビッグデータ分析とBIツール開発を行う企業。データサイエンティストとして成長できる。",
      matchScore: 0,
      tags: ["データ分析", "成長企業", "リモート可"],
      techStack: ["Python", "SQL", "Tableau", "Spark"],
      projectTypes: ["データ分析", "BI開発"],
      salary: "450-700万円",
      benefits: ["リモートワーク可", "フレックスタイム", "データサイエンス研修", "カンファレンス参加費"],
      culture: ["データドリブン", "自由な発想", "成果主義", "学習環境充実"],
      founded: "2017年",
      website: "https://data-analytics.example.com",
      parentCompany: undefined,
      subsidiaries: [],
      partnerships: ["Tableau", "Snowflake", "Databricks"],
      capitalStructure: {
        shareholders: [
          { name: "創業者", percentage: 50 },
          { name: "ベンチャーキャピタルB", percentage: 35 },
          { name: "事業会社", percentage: 15 },
        ],
      },
    },
    {
      id: 6,
      name: "モバイルアプリ開発株式会社",
      industry: "モバイルアプリ開発",
      location: "東京都渋谷区",
      employees: "80名",
      size: "ベンチャー企業（100-300名）",
      description: "iOS/Androidアプリの受託開発を行うベンチャー。自由な社風とチーム開発重視。",
      matchScore: 0,
      tags: ["モバイル", "ベンチャー", "チーム開発"],
      techStack: ["Swift", "Kotlin", "Flutter"],
      projectTypes: ["アプリ開発", "受託開発"],
      salary: "400-600万円",
      benefits: ["リモートワーク可", "フレックスタイム", "最新デバイス貸与", "勉強会参加費"],
      culture: ["自由な社風", "チーム重視", "アジャイル開発", "ユーザーファースト"],
      founded: "2019年",
      website: "https://mobile-app-dev.example.com",
      parentCompany: undefined,
      subsidiaries: [],
      partnerships: ["Apple", "Google", "Firebase"],
      capitalStructure: {
        shareholders: [
          { name: "創業者グループ", percentage: 80 },
          { name: "エンジェル投資家", percentage: 15 },
          { name: "従業員持株会", percentage: 5 },
        ],
      },
    },
    {
      id: 7,
      name: "エンタープライズシステムズ株式会社",
      industry: "社内システム開発",
      location: "神奈川県横浜市",
      employees: "500名",
      size: "中堅企業（300-800名）",
      description: "大手メーカーのグループ会社。安定した環境で社内システムの開発・運用を担当。",
      matchScore: 0,
      tags: ["安定性", "ワークライフバランス", "福利厚生"],
      techStack: ["Java", "C#", ".NET"],
      projectTypes: ["社内システム", "業務システム"],
      salary: "380-580万円",
      benefits: ["住宅手当", "家族手当", "退職金制度", "社員食堂", "保養所", "育児支援"],
      culture: ["安定志向", "ワークライフバランス", "長期雇用", "チームワーク重視"],
      founded: "1995年",
      website: "https://ent-systems.example.co.jp",
      parentCompany: "大手製造業グループ",
      subsidiaries: [],
      partnerships: ["Microsoft", "Oracle"],
      capitalStructure: {
        shareholders: [
          { name: "親会社", percentage: 100 },
        ],
      },
    },
    {
      id: 8,
      name: "株式会社ゲームテクノロジー",
      industry: "ゲーム開発",
      location: "東京都中野区",
      employees: "200名",
      size: "中堅企業（300-800名）",
      description: "スマホゲームの開発・運営を行う企業。エンタメ×技術で楽しく働ける環境。",
      matchScore: 0,
      tags: ["ゲーム", "クリエイティブ", "自社サービス"],
      techStack: ["Unity", "C#", "Go"],
      projectTypes: ["ゲーム開発", "自社サービス"],
      salary: "420-650万円",
      benefits: ["フレックスタイム", "リモートワーク可", "ゲーム購入補助", "社内イベント充実"],
      culture: ["クリエイティブ", "楽しさ重視", "ユーザー体験第一", "チーム協力"],
      founded: "2014年",
      website: "https://game-tech.example.com",
      parentCompany: undefined,
      subsidiaries: ["GT Studio"],
      partnerships: ["Unity Technologies", "Google Play", "App Store"],
      capitalStructure: {
        shareholders: [
          { name: "創業者グループ", percentage: 40 },
          { name: "ゲーム大手D", percentage: 35 },
          { name: "ベンチャーキャピタル", percentage: 25 },
        ],
      },
    },
    {
      id: 9,
      name: "フィンテック株式会社",
      industry: "金融×IT",
      location: "東京都千代田区",
      employees: "250名",
      size: "中堅企業（300-800名）",
      description: "金融業界向けのITソリューションを提供。高い技術力と金融知識を身につけられる。",
      matchScore: 0,
      tags: ["金融IT", "高給与", "成長分野"],
      techStack: ["Java", "Python", "Blockchain"],
      projectTypes: ["金融システム", "フィンテック"],
      salary: "500-800万円",
      benefits: ["高給与", "リモートワーク可", "資格取得支援", "金融知識研修", "福利厚生充実"],
      culture: ["高い専門性", "コンプライアンス重視", "技術革新", "成長志向"],
      founded: "2012年",
      website: "https://fintech.example.co.jp",
      parentCompany: undefined,
      subsidiaries: ["FT Blockchain Lab"],
      partnerships: ["大手銀行E", "証券会社F"],
      capitalStructure: {
        shareholders: [
          { name: "創業者", percentage: 35 },
          { name: "金融機関E", percentage: 40 },
          { name: "ベンチャーキャピタル", percentage: 25 },
        ],
      },
    },
    {
      id: 10,
      name: "株式会社AIリサーチラボ",
      industry: "AI研究開発",
      location: "東京都文京区",
      employees: "60名",
      size: "スタートアップ（50名以下）",
      description: "最先端のAI研究を行うスタートアップ。論文執筆や学会発表の機会も豊富。",
      matchScore: 0,
      tags: ["AI研究", "スタートアップ", "最先端技術"],
      techStack: ["Python", "TensorFlow", "PyTorch"],
      projectTypes: ["研究開発", "AI開発"],
      salary: "450-750万円",
      benefits: ["フレックスタイム", "リモートワーク可", "学会参加費", "論文執筆支援", "最新GPU環境"],
      culture: ["研究志向", "最先端追求", "自由な発想", "学術的"],
      founded: "2020年",
      website: "https://ai-research-lab.example.com",
      parentCompany: undefined,
      subsidiaries: [],
      partnerships: ["大学G研究室", "AI研究機関H"],
      capitalStructure: {
        shareholders: [
          { name: "創業者グループ", percentage: 50 },
          { name: "大学ファンド", percentage: 30 },
          { name: "ベンチャーキャピタル", percentage: 20 },
        ],
      },
    },
  ]
}

/** AIスコアを使用して企業を生成（バックエンドAI連携のフォールバック） */
export const generateITCompanies = (userData: UserData): Company[] => {
  const companies = getBaseCompanyData()

  // userDataからスコアマップを作成
  const scoreMap: UserScore = {}
  if (userData.scores) {
    userData.scores.forEach(s => {
      scoreMap[s.category] = s.score
    })
  }

  return companies
    .map((company) => {
      let score = 50 // ベーススコア

      // AIが分析したスコアに基づいてマッチング

      // 1. 技術志向スコア
      const techScore = scoreMap["技術志向"] || 0
      if (techScore > 5 && company.tags.includes("技術力重視")) {
        score += techScore * 1.5
      }
      if (techScore > 7 && company.industry.includes("AI")) {
        score += 10
      }

      // 2. コミュニケーション能力
      const commScore = scoreMap["コミュニケーション能力"] || 0
      if (commScore > 5 && company.tags.includes("チーム開発")) {
        score += commScore * 1.2
      }
      if (commScore > 7 && company.projectTypes.includes("大規模プロジェクト")) {
        score += 8
      }

      // 3. リーダーシップ
      const leaderScore = scoreMap["リーダーシップ"] || 0
      if (leaderScore > 5 && company.culture.includes("リーダーシップ育成")) {
        score += leaderScore * 1.3
      }
      if (leaderScore > 7 && company.size === "大企業") {
        score += 10
      }

      // 4. チームワーク
      const teamScore = scoreMap["チームワーク"] || 0
      if (teamScore > 5 && company.tags.includes("チーム開発")) {
        score += teamScore * 1.2
      }

      // 5. 問題解決力
      const problemScore = scoreMap["問題解決力"] || 0
      if (problemScore > 5 && company.industry.includes("コンサル")) {
        score += problemScore * 1.5
      }
      if (problemScore > 7 && company.tags.includes("技術力重視")) {
        score += 10
      }

      // 6. 創造性・発想力
      const creativityScore = scoreMap["創造性・発想力"] || 0
      if (creativityScore > 5 && company.size === "ベンチャー") {
        score += creativityScore * 1.5
      }
      if (creativityScore > 7 && company.projectTypes.includes("自社サービス")) {
        score += 12
      }

      // 7. 計画性・実行力
      const planningScore = scoreMap["計画性・実行力"] || 0
      if (planningScore > 5 && company.industry.includes("SIer")) {
        score += planningScore * 1.3
      }

      // 8. 学習意欲・成長志向
      const growthScore = scoreMap["学習意欲・成長志向"] || 0
      if (growthScore > 5 && company.tags.includes("教育制度充実")) {
        score += growthScore * 1.4
      }
      if (growthScore > 7 && company.size === "ベンチャー") {
        score += 10
      }

      // 9. ストレス耐性・粘り強さ
      const stressScore = scoreMap["ストレス耐性・粘り強さ"] || 0
      if (stressScore > 5 && company.projectTypes.includes("大規模プロジェクト")) {
        score += stressScore * 1.2
      }
      if (stressScore < 3 && company.tags.includes("ワークライフバランス")) {
        score += 10
      }

      // 10. ビジネス思考・目標志向
      const businessScore = scoreMap["ビジネス思考・目標志向"] || 0
      if (businessScore > 5 && company.projectTypes.includes("自社サービス")) {
        score += businessScore * 1.4
      }
      if (businessScore > 7 && company.industry.includes("コンサル")) {
        score += 12
      }

      return { ...company, matchScore: Math.min(score, 99) }
    })
    .sort((a, b) => b.matchScore - a.matchScore)
    .slice(0, 10) // 上位10社に絞り込み
}
