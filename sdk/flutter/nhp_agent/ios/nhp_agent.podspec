#
# OpenNHP Agent Flutter Plugin - iOS CocoaPods Spec
#
# To install, add this to your Podfile:
#   pod 'nhp_agent', :path => '../opennhp/sdk/flutter/nhp_agent'
#

Pod::Spec.new do |s|
  s.name             = 'nhp_agent'
  s.version          = '0.1.0'
  s.summary          = 'OpenNHP Agent SDK for Flutter'
  s.description      = <<-DESC
OpenNHP Agent SDK for Flutter. Provides Network-resource Hiding Protocol (NHP)
functionality for Zero Trust network access on iOS.
                       DESC
  s.homepage         = 'https://github.com/OpenNHP/opennhp'
  s.license          = { :file => '../LICENSE' }
  s.author           = { 'OpenNHP' => 'info@opennhp.org' }
  s.source           = { :path => '.' }
  s.source_files     = 'Classes/**/*'
  s.dependency 'Flutter'
  s.platform         = :ios, '12.0'

  # The NhpAgent.xcframework is built from Go using gomobile bind
  # Run scripts/build_native.sh to generate it
  s.vendored_frameworks = 'Frameworks/NhpAgent.xcframework'

  # Flutter.framework does not contain a i386 slice.
  s.pod_target_xcconfig = { 'DEFINES_MODULE' => 'YES', 'EXCLUDED_ARCHS[sdk=iphonesimulator*]' => 'i386' }
  s.swift_version = '5.0'
end
